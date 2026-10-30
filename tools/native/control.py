#!/usr/bin/env python3
"""Exercise external controller/receiver recovery in a fresh, private Linux lab."""
import argparse
import hashlib
import http.client
import json
import os
import pathlib
import sqlite3
import subprocess
import time
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lab-root", required=True, type=pathlib.Path)
    parser.add_argument("--attempt", required=True)
    args = parser.parse_args()
    root = args.lab_root.resolve()
    assert str(root / "lab-server.py").encode() + b"\x00" in pathlib.Path("/proc/1/cmdline").read_bytes(), "Run only inside the owned lab"
    assert args.attempt.isalnum(), "Use an alphanumeric, fresh attempt name"
    state = root / ("control-" + args.attempt)
    state.mkdir(mode=0o700)
    runtime = pathlib.Path("/home/ttstack") / ("control-" + args.attempt)
    runtime.mkdir(mode=0o700)
    key = uuid.uuid4().hex + uuid.uuid4().hex
    key_path = state / "api-key"
    descriptor = os.open(key_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as stream:
        stream.write(key)
    authority = str(uuid.uuid4())
    epoch = 1
    sequence = 0
    services = {}
    evidence = []
    host_id = "control-native-" + args.attempt
    vm_id = "control-vm-" + str(uuid.uuid4())
    env_id = "control-env-" + str(uuid.uuid4()) + "-g1"
    volume_id = "control-data-" + str(uuid.uuid4()) + "-g1"
    authority_paths = {name: state / (name + "-authority.json") for name in ("agent", "ctl")}

    def record(label, **details):
        evidence.append(dict(check=label, **details))
        (state / "progress.json").write_text(json.dumps(evidence, indent=2) + "\n")
        print(label, flush=True)

    def write_authorities():
        for path in authority_paths.values():
            temporary = path.with_suffix(".next")
            with temporary.open("x") as stream:
                os.fchmod(stream.fileno(), 0o600)
                json.dump(dict(version=1, authority=authority, epoch=epoch, phase="fenced", key=str(uuid.uuid4())), stream)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, path)
        descriptor = os.open(state, os.O_DIRECTORY)
        os.fsync(descriptor)
        os.close(descriptor)

    def call(name, method, path, body=None, subject=None, replay=None, error=None):
        nonlocal sequence
        raw = json.dumps(body).encode() if body is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        command = None
        if method != "GET":
            sequence += 1
            subject = subject or "control-" + authority + "-g" + str(epoch)
            command = replay or json.dumps(dict(authority=authority, epoch=epoch, subject=subject,
                generation=int(subject.rsplit("-g", 1)[1]), token=(epoch << 40) | sequence,
                key=str(uuid.uuid4()), fingerprint=hashlib.sha256(method.encode() + path.encode() + (raw or b"")).hexdigest()))
            headers["X-Compute-Command"] = command
        connection = http.client.HTTPConnection("127.0.0.1", 19100 if name == "agent" else 19200, timeout=150)
        try:
            connection.request(method, path, raw, headers)
            response = connection.getresponse()
            value = json.loads(response.read())
        finally:
            connection.close()
        if error:
            assert response.status >= 400 and value.get("failure", {}).get("code") == error, (name, method, path, response.status, value, error)
        else:
            assert 200 <= response.status < 300 and value["ok"], (name, method, path, response.status, value)
        return value.get("data"), command

    def start(name):
        flags = ["--listen", "127.0.0.1:" + ("19100" if name == "agent" else "19200"),
            "--data-dir", str(state / name), "--authority-file", str(authority_paths[name])]
        if name == "agent":
            flags += ["--host-id", host_id, "--image-dir", str(root / "images"), "--runtime-dir", str(runtime),
                "--storage", "file", "--cpu-total", "2", "--mem-total", "1024", "--disk-total", "4096"]
        log = (state / (name + ".log")).open("a")
        process = subprocess.Popen([str(root / "bin" / ("tt-agent" if name == "agent" else "tt-ctl"))] + flags,
            env=dict(os.environ, TT_API_KEY=key, GOMAXPROCS="2"), stdout=log, stderr=log)
        services[name] = process, log
        deadline = time.monotonic() + 40
        while True:
            assert process.poll() is None, name + " exited; inspect its private log"
            try:
                call(name, "GET", "/api/control")
                return
            except (OSError, http.client.HTTPException):
                assert time.monotonic() < deadline, name + " did not become ready"
                time.sleep(.2)

    def stop(name):
        process, log = services.pop(name)
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=45)
        log.close()
        assert process.returncode == 0, name + " shutdown failed"

    def inventory():
        snapshot, _ = call("agent", "GET", "/api/control/inventory?size=1")
        items = list(snapshot["items"])
        cursor = snapshot.get("next_token")
        while cursor:
            page, _ = call("agent", "GET", "/api/control/inventory?size=1&snapshot=" + snapshot["id"] + "&after=" + cursor)
            assert page["digest"] == snapshot["digest"] and page["id"] == snapshot["id"] and len(page["items"]) <= 1
            items.extend(page["items"])
            cursor = page.get("next_token")
        assert len(items) == snapshot["total"] == len({(item["kind"], item["id"]) for item in items})
        assert snapshot["complete"] and snapshot["frozen"] and snapshot["processes"] == 0
        call("agent", "GET", "/api/control/inventory?snapshot=" + snapshot["id"] + "&after=" + str(snapshot["total"] + 1), error="invalid_page")
        return snapshot, items

    def fence(name):
        receipt, command = call(name, "POST", "/api/control/fence", {})
        replayed, _ = call(name, "POST", "/api/control/fence", {}, replay=command)
        assert replayed == receipt
        return receipt

    def resume(name, phase, receipt, previous, plan, catalog, snapshot=None, error=None, replay=None):
        body = dict(phase=phase, fence_id=receipt["id"], previous=previous, catalog_digest=catalog, plan_digest=plan)
        if snapshot:
            body["inventory_digest"] = snapshot["digest"]
        result, command = call(name, "POST", "/api/control/resume", body, error=error, replay=replay)
        if not error:
            repeated, _ = call(name, "POST", "/api/control/resume", body, replay=command)
            assert repeated == result
        return result, command

    def disk_used():
        info, _ = call("agent", "GET", "/api/info")
        return info["resource"]["disk_used"]

    def alive(pid, identity):
        try:
            return hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest() == identity
        except FileNotFoundError:
            return False

    write_authorities()
    try:
        start("agent")
        start("ctl")
        orphan = runtime / ("volume-orphan-" + str(uuid.uuid4()) + "-g1")
        with orphan.open("xb") as stream:
            stream.truncate(32 << 20)
        orphan_identity = orphan.stat().st_ino
        (state / "ownership.json").write_text(json.dumps(dict(runtime=str(runtime), orphan=str(orphan), inode=orphan_identity, vm_id=vm_id, env_id=env_id, volume_id=volume_id)))
        call("agent", "POST", "/api/volumes", dict(id=volume_id, generation=1, size_mib=4, retain=False), subject=volume_id, error="recovery_fenced")
        for query in ("size=0", "size=101", "after=1", "after=-1"):
            call("agent", "GET", "/api/control/inventory?" + query, error="invalid_page")
        af = fence("agent")
        cf = fence("ctl")
        snapshot, items = inventory()
        assert snapshot["quarantine_mib"] == 32 and not snapshot["quarantine_pinned"], snapshot
        plan = hashlib.sha256((authority + "/review1").encode()).hexdigest()
        catalog = hashlib.sha256(json.dumps(items, sort_keys=True).encode()).hexdigest()
        ar, _ = resume("agent", "reads", af, af["id"], plan, catalog, snapshot)
        changed = runtime / ("volume-changed-" + str(uuid.uuid4()) + "-g1")
        with changed.open("xb") as stream:
            stream.truncate(1 << 20)
        _, retry = resume("agent", "mutations", af, ar["id"], plan, catalog, snapshot, error="inventory_changed")
        changed.unlink()
        am, _ = resume("agent", "mutations", af, ar["id"], plan, catalog, snapshot, replay=retry)
        cr, _ = resume("ctl", "reads", cf, cf["id"], plan, catalog)
        cm, _ = resume("ctl", "mutations", cf, cr["id"], plan, catalog)
        assert disk_used() == 32
        call("agent", "POST", "/api/volumes", dict(id=volume_id, generation=1, size_mib=4065, retain=False), subject=volume_id, error="capacity_exceeded")
        assert orphan.stat().st_ino == orphan_identity
        orphan.unlink()
        assert disk_used() == 32, "Removing a physical orphan released an external claim"
        stop("agent")
        start("agent")
        assert disk_used() == 32, "Receiver restart lost the quarantine floor"
        record("Frozen paged inventory, changed-inventory refusal, exact retry, quarantine admission and restart persistence passed", quarantine_mib=32)

        created, create_command = call("agent", "POST", "/api/vms", dict(vm_id=vm_id, env_id=env_id,
            image="alpine-cloud", engine="qemu", cpu=1, mem=256, disk=1024, ports=[], deny_outgoing=False), subject=env_id)
        assert created["vm"]["state"] == "running", created
        pid = int((pathlib.Path("/home/ttstack/run") / ("qemu-" + vm_id + ".pid")).read_text())
        identity = hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest()
        assert vm_id.encode() in (pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()
        report, _ = call("agent", "GET", "/api/control")
        enrollment = dict(host_id=host_id, endpoint="http://127.0.0.1:19100", scope_digest=report["scope_digest"], plan_digest=plan)
        call("ctl", "POST", "/api/hosts", dict(addr="127.0.0.1:19100"), error="unregistered_mutation")
        call("ctl", "POST", "/api/control/enroll", dict(enrollment, plan_digest="0" * 64), error="recovery_plan_conflict")
        call("ctl", "POST", "/api/control/enroll", dict(enrollment, scope_digest="0" * 64), error="identity_mismatch")
        call("ctl", "POST", "/api/control/enroll", dict(enrollment, host_id="wrong-host"), error="identity_mismatch")
        enrolled, enroll_command = call("ctl", "POST", "/api/control/enroll", enrollment)
        repeated, _ = call("ctl", "POST", "/api/control/enroll", enrollment, replay=enroll_command)
        assert enrolled == repeated
        call("ctl", "POST", "/api/control/enroll", dict(enrollment, host_id="wrong-host"), replay=enroll_command, error="fingerprint_conflict")
        envs, _ = call("ctl", "GET", "/api/envs")
        assert not envs and alive(pid, identity)
        with sqlite3.connect(state / "ctl" / "ctl.db") as db:
            assert db.execute("SELECT COUNT(*) FROM vms").fetchone()[0] == 0
        record("Explicit controller enrollment, stable replay, plan/scope/identity refusal and no adoption of a running agent VM passed")

        old_inventory, _ = call("agent", "GET", "/api/control/inventory?size=1")
        epoch = 2
        write_authorities()
        call("agent", "GET", "/api/control/inventory?snapshot=" + old_inventory["id"], error="inventory_expired")
        # Control identity embeds the epoch; its obsolete subject is rejected
        # before admission. Workload identities remain stable across epochs.
        call("ctl", "POST", "/api/control/enroll", enrollment, replay=enroll_command, error="invalid_identity")
        call("agent", "POST", "/api/vms", dict(vm_id=vm_id, env_id=env_id,
            image="alpine-cloud", engine="qemu", cpu=1, mem=256, disk=1024, ports=[], deny_outgoing=False), subject=env_id, replay=create_command, error="stale_epoch")
        assert alive(pid, identity), "Credential fencing was incorrectly counted as physical fencing"
        cf = fence("ctl")
        assert cf["method"] == "controller_commands_drained" and alive(pid, identity)
        af = fence("agent")
        assert af["method"] == "owned_processes_stopped" and not alive(pid, identity)
        snapshot, items = inventory()
        assert all(item["id"] != orphan.name.removeprefix("volume-") for item in items)
        # QEMU's conservative capacity envelope may exceed its VM claim. The
        # resolved orphan disappears, while remaining physical overhead stays
        # reserved rather than being mistaken for a leaked orphan claim.
        assert snapshot["quarantine_mib"] == sum(max(0, item.get("capacity_mib", 0) - item.get("claim_mib", 0)) for item in items)
        assert not snapshot["quarantine_pinned"]
        plan = hashlib.sha256((authority + "/review2").encode()).hexdigest()
        catalog = hashlib.sha256(json.dumps(items, sort_keys=True).encode()).hexdigest()
        ar, _ = resume("agent", "reads", af, af["id"], plan, catalog, snapshot)
        am, _ = resume("agent", "mutations", af, ar["id"], plan, catalog, snapshot)
        cr, _ = resume("ctl", "reads", cf, cf["id"], plan, catalog)
        cm, _ = resume("ctl", "mutations", cf, cr["id"], plan, catalog)
        enrollment.update(scope_digest=af["scope_digest"], plan_digest=plan)
        call("ctl", "POST", "/api/control/enroll", enrollment)
        record("New epoch expires pages and old successes; controller barrier retains guest; receiver physical fence stops it", quarantine_mib=snapshot["quarantine_mib"])

        call("ctl", "POST", "/api/volumes", dict(id=volume_id, host_id=host_id, generation=1, size_mib=4, retain=False), subject=volume_id)
        call("ctl", "DELETE", "/api/volumes/" + volume_id, subject=volume_id, error="gc_fenced")
        call("agent", "DELETE", "/api/vms/" + vm_id, subject=env_id, error="gc_fenced")
        cg, _ = resume("ctl", "gc", cf, cm["id"], plan, catalog)
        call("ctl", "DELETE", "/api/volumes/" + volume_id, subject=volume_id, error="gc_fenced")
        ag, _ = resume("agent", "gc", af, am["id"], plan, catalog, snapshot)
        assert ag["quarantine_mib"] == am["quarantine_mib"]
        call("ctl", "DELETE", "/api/volumes/" + volume_id, subject=volume_id)
        call("agent", "DELETE", "/api/vms/" + vm_id, subject=env_id)
        for name in ("agent", "ctl"):
            stop(name)
            start(name)
            report, _ = call(name, "GET", "/api/control")
            assert report["phase"] == "gc" and report["epoch"] == 2 and len(report["activations"]) == 3
        assert not call("agent", "GET", "/api/vms")[0]
        assert not call("ctl", "GET", "/api/envs")[0]
        assert not list(runtime.iterdir()), list(runtime.iterdir())
        record("Both controller and receiver GC gates, forwarded volume deletion, VM deletion and persisted activation chain passed")
        retained_floor = disk_used()
        assert retained_floor == ag["quarantine_mib"]
        epoch = 3
        write_authorities()
        af, cf = fence("agent"), fence("ctl")
        snapshot, items = inventory()
        assert snapshot["quarantine_mib"] == 0 and not snapshot["quarantine_pinned"]
        plan = hashlib.sha256((authority + "/empty-review3").encode()).hexdigest()
        catalog = hashlib.sha256(json.dumps(items, sort_keys=True).encode()).hexdigest()
        for name, receiver_fence, physical in (("agent", af, snapshot), ("ctl", cf, None)):
            previous = receiver_fence["id"]
            for phase in ("reads", "mutations", "gc"):
                receipt, _ = resume(name, phase, receiver_fence, previous, plan, catalog, physical)
                previous = receipt["id"]
        assert disk_used() == 0
        record("Only a new reviewed empty epoch releases the retained physical-capacity floor", previous_floor_mib=retained_floor)
        (state / "result.json").write_text(json.dumps(dict(passed=True, checks=evidence), indent=2) + "\n")
    finally:
        # Service termination retains disks on failure for explicit inspection.
        for name in list(services):
            stop(name)
        key_path.unlink()


if __name__ == "__main__":
    main()
