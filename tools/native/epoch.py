#!/usr/bin/env python3
"""Validate an external receiver epoch and physical fencing after SQLite rollback."""
import argparse
import hashlib
import json
import os
import pathlib
import select
import shutil
import signal
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--agent", required=True)
    p.add_argument("--unit", required=True)
    p.add_argument("--key-file", required=True)
    p.add_argument("--authority-file", required=True)
    p.add_argument("--data-dir", required=True)
    p.add_argument("--runtime-dir", required=True)
    p.add_argument("--run-dir", required=True)
    p.add_argument("--image", required=True)
    p.add_argument("--engine", choices=("firecracker", "qemu"), default="firecracker")
    p.add_argument("--preserve-unit", action="append", default=[])
    args = p.parse_args()
    key = pathlib.Path(args.key_file).read_text().strip()
    authority_path = pathlib.Path(args.authority_file)
    authority = json.loads(authority_path.read_text())
    assert authority["epoch"] == 1 and authority["phase"] == "fenced", "A fresh disposable receiver must begin fenced"
    run = str(uuid.uuid4())
    environment, vm_id = "epoch-" + run + "-g1", "epoch-vm-" + run
    db = pathlib.Path(args.data_dir) / "agent.db"
    root = "epoch-root-" + run + "-g1"
    root_created = False
    data_id = "epoch-data-" + run + "-g1"
    data_created = False
    pid = None
    identity_digest = None
    sequence = 0
    preserved = {unit: subprocess.check_output(["systemctl", "show", unit, "-p", "MainPID", "--value"], text=True).strip() for unit in args.preserve_unit}

    def wait_ready():
        for _ in range(120):
            try:
                call("GET", "/api/control")
                return
            except (urllib.error.URLError, ConnectionError):
                time.sleep(.25)
        raise RuntimeError("Scoped epoch receiver did not become ready")

    def call(method, path, body=None, subject=None, epoch=None, generation=1, allowed=(200, 201), replay=None):
        nonlocal sequence
        raw = json.dumps(body).encode() if body is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        if method != "GET":
            sequence += 1
            epoch = authority["epoch"] if epoch is None else epoch
            headers["X-Compute-Command"] = replay or json.dumps({"authority": authority["authority"], "epoch": epoch,
                "subject": subject, "generation": generation, "token": (epoch << 40) | sequence,
                "key": str(uuid.uuid4()), "fingerprint": hashlib.sha256(method.encode() + path.encode() + (raw or b"")).hexdigest()})
        request = urllib.request.Request(args.agent + path, data=raw, method=method, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=150) as response:
                status, value = response.status, json.load(response)
        except urllib.error.HTTPError as error:
            status, value = error.code, json.load(error)
        if status not in allowed:
            raise RuntimeError("Unexpected scoped receiver result: " + json.dumps({"status": status, "result": value}))
        return value, headers.get("X-Compute-Command")

    def snapshot(path):
        with sqlite3.connect(db) as source, sqlite3.connect(path) as target:
            source.backup(target)

    def active():
        if not pid:
            return False
        try:
            raw = pathlib.Path("/proc") / str(pid) / "cmdline"
            return hashlib.sha256(raw.read_bytes()).hexdigest() == identity_digest and bool(raw.stat())
        except FileNotFoundError:
            return False

    try:
        wait_ready()
        # Bootstrap through receiver-owned evidence. A serving-mode file hint
        # cannot bypass the physical fence and frozen-inventory phase chain.
        control_subject = "control-" + authority["authority"] + "-g1"
        fenced, _ = call("POST", "/api/control/fence", {}, control_subject)
        fence = fenced["data"]
        captured, _ = call("GET", "/api/control/inventory?size=100")
        captured = captured["data"]
        items = list(captured["items"])
        cursor = captured.get("next_token")
        while cursor:
            page, _ = call("GET", "/api/control/inventory?snapshot=" + captured["id"] + "&after=" + cursor)
            assert page["data"]["digest"] == captured["digest"]
            items.extend(page["data"]["items"])
            cursor = page["data"].get("next_token")
        assert captured["complete"] and captured["frozen"] and captured["processes"] == 0 and len(items) == captured["total"]
        catalog_digest = hashlib.sha256(json.dumps(items, sort_keys=True).encode()).hexdigest()
        plan_digest = hashlib.sha256((authority["authority"] + "/" + fence["host_id"]).encode()).hexdigest()
        previous = fence["id"]
        for phase in ("reads", "mutations"):
            activated, _ = call("POST", "/api/control/resume", {"phase": phase, "fence_id": fence["id"],
                "previous": previous, "inventory_digest": captured["digest"], "catalog_digest": catalog_digest,
                "plan_digest": plan_digest}, control_subject)
            previous = activated["data"]["id"]
        with tempfile.TemporaryDirectory(prefix="epoch-recovery-") as snapshots:
            before = pathlib.Path(snapshots) / "before.db"
            snapshot(before)
            call("POST", "/api/volumes", {"id": data_id, "generation": 1, "size_mib": 16, "retain": False}, data_id)
            data_created = True
            marker = "epoch-fence-" + run
            with (pathlib.Path(args.runtime_dir) / ("volume-" + data_id)).open("r+b") as stream:
                stream.seek(4096)
                stream.write(marker.encode())
                stream.flush()
                os.fsync(stream.fileno())
            created, original_command = call("POST", "/api/vms", {"vm_id": vm_id, "env_id": environment,
                "image": args.image, "engine": args.engine, "cpu": 1, "mem": 256, "disk": 1024, "ports": [], "deny_outgoing": False,
                "durable_root": True, "root_volume_id": root, "data_volume_ids": [data_id], "root_retain": False, "initial_stopped": True, "isolated_network": True}, environment)
            root_created = True
            vm = created["data"]["vm"]
            root = vm["options"]["root_volume_id"]
            call("POST", "/api/vms/" + vm_id + "/start", {}, environment)
            pid = int((pathlib.Path(args.run_dir) / (("fc-" if args.engine == "firecracker" else "qemu-") + vm_id + ".pid")).read_text())
            identity_digest = hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest()
            assert active(), "The native guest was not running"
            descriptor = os.pidfd_open(pid)
            assert hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest() == identity_digest
            signal.pidfd_send_signal(descriptor, signal.SIGKILL)
            poller = select.poll();poller.register(descriptor, select.POLLIN)
            assert poller.poll(15000), "Owned surprise-exit fixture did not stop"
            os.close(descriptor)
            observed, _ = call("GET", "/api/vms/" + vm_id)
            assert observed["data"]["state"] == "stopped", "A passive snapshot retained stale process liveness"
            with sqlite3.connect(db) as source:
                stored = json.loads(source.execute("SELECT data FROM vms WHERE id=?", (vm_id,)).fetchone()[0])
            assert stored["state"] == "running", "Observation rewrote durable intent"
            info, _ = call("GET", "/api/info")
            assert info["data"]["resource"]["cpu_used"] >= 1, "Observation released an accepted computation claim"
            call("POST", "/api/vms/" + vm_id + "/stop", {}, environment)
            call("POST", "/api/vms/" + vm_id + "/start", {}, environment)
            pid = int((pathlib.Path(args.run_dir) / (("fc-" if args.engine == "firecracker" else "qemu-") + vm_id + ".pid")).read_text())
            identity_digest = hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest()
            assert active(), "Fresh explicit operation could not restart the stopped fixture"
            subprocess.run(["systemctl", "stop", args.unit], check=True)
            assert active(), "Service shutdown did not preserve the predecessor needed for the drill"
            authority.update(epoch=2, phase="fenced", key=str(uuid.uuid4()))
            temporary = authority_path.with_suffix(".next")
            temporary.write_text(json.dumps(authority))
            temporary.chmod(0o600)
            with temporary.open("rb") as source:
                os.fsync(source.fileno())
            os.replace(temporary, authority_path)
            directory_fd = os.open(authority_path.parent, os.O_DIRECTORY)
            os.fsync(directory_fd)
            os.close(directory_fd)
            with sqlite3.connect(before) as source, sqlite3.connect(db) as target:
                source.backup(target)
            subprocess.run(["systemctl", "start", args.unit], check=True)
            wait_ready()
            stale, _ = call("POST", "/api/vms", {"vm_id": vm_id, "env_id": environment,
                "image": args.image, "engine": args.engine, "cpu": 1, "mem": 256, "disk": 1024, "ports": [], "deny_outgoing": False,
                "durable_root": True, "root_volume_id": root, "data_volume_ids": [data_id], "root_retain": False, "initial_stopped": True, "isolated_network": True}, environment, allowed=(409,), replay=original_command)
            assert stale["failure"]["code"] == "stale_epoch", stale
            assert active(), "Epoch refusal itself was incorrectly counted as physical fencing"
            control_subject = "control-" + authority["authority"] + "-g2"
            diagnostics = {"pid": pid, "argv0": (pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes().split(b"\x00")[0].decode(errors="replace")}
            diagnostics["cgroup"] = (pathlib.Path("/proc") / str(pid) / "cgroup").read_text()
            for field in ("root", "exe", "ns/mnt"):
                try:
                    diagnostics[field] = os.readlink(pathlib.Path("/proc") / str(pid) / field)
                except OSError as error:
                    diagnostics[field] = {"errno": error.errno}
            evidence_path = pathlib.Path(args.data_dir) / "fence-diagnostics.json"
            evidence_path.write_text(json.dumps(diagnostics));evidence_path.chmod(0o600)
            fenced, fence_command = call("POST", "/api/control/fence", {}, control_subject, generation=2)
            receipt = fenced["data"]
            assert receipt["method"] == "owned_processes_stopped" and receipt["known_vms"] == 0, receipt
            assert not active(), "An untracked predecessor survived its fencing receipt"
            replayed, _ = call("POST", "/api/control/fence", {}, control_subject, generation=2, replay=fence_command)
            assert replayed["data"]["id"] == receipt["id"], "Fencing replay changed its receipt"
            control, _ = call("GET", "/api/control")
            assert control["data"]["phase"] == "inventory" and control["data"]["fence"]["id"] == receipt["id"]
            inventory, _ = call("GET", "/api/control/inventory?size=100")
            items = inventory["data"]["items"]
            cursor = inventory["data"].get("next_token")
            while cursor:
                page, _ = call("GET", "/api/control/inventory?snapshot=" + inventory["data"]["id"] + "&after=" + cursor)
                assert page["data"]["digest"] == inventory["data"]["digest"]
                items.extend(page["data"]["items"])
                cursor = page["data"].get("next_token")
            assert inventory["data"]["processes"] == 0 and inventory["data"]["frozen"] and inventory["data"]["complete"]
            assert any(item["kind"] == "volume" and item["id"] == root and item["physical"] and not item["metadata"] for item in items), "Restored metadata concealed the orphan disk"
            with (pathlib.Path(args.runtime_dir) / ("volume-" + data_id)).open("rb") as stream:
                stream.seek(4096)
                assert stream.read(len(marker)) == marker.encode(), "Fencing destroyed predecessor data"
            for unit, expected in preserved.items():
                actual = subprocess.check_output(["systemctl", "show", unit, "-p", "MainPID", "--value"], text=True).strip()
                assert actual == expected, "A service outside this scope was restarted"
            print(json.dumps({"result": "PASS", "run": run, "fence_receipt": receipt["id"], "engine": args.engine, "checks": ["surprise exit observed without intent rewrite", "accepted compute claim retained on observation", "fresh explicit restart", "real predecessor survives receiver restart", "SQLite rollback loses guest record", "external epoch refuses stale command", "fencing distinct from credential refusal", "pidfd stops untracked scoped process", "fencing receipt replay", "physical orphan inventory", "data preserved", "unrelated services preserved"]}))
    finally:
        stopped = subprocess.run(["systemctl", "stop", args.unit], check=False)
        if active():
            descriptor = os.pidfd_open(pid)
            if hashlib.sha256((pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes()).hexdigest() != identity_digest:
                os.close(descriptor)
                raise RuntimeError("Cleanup identity changed; scoped resources retained")
            signal.pidfd_send_signal(descriptor, signal.SIGKILL)
            poller = select.poll();poller.register(descriptor, select.POLLIN)
            if not poller.poll(15000):
                os.close(descriptor)
                raise RuntimeError("Owned guest exit not confirmed; data retained")
            os.close(descriptor)
        if stopped.returncode and subprocess.check_output(["systemctl","show",args.unit,"-p","ActiveState","--value"],text=True).strip() not in ("", "inactive", "failed"):
            raise RuntimeError("Disposable receiver did not stop; retain namespace bytes")
        if root_created:
            sandbox_path = pathlib.Path(args.run_dir) / ("fc-" + vm_id + ".sandbox.json")
            if sandbox_path.exists():
                sandbox = json.loads(sandbox_path.read_text())
                jail = pathlib.Path(sandbox["root"])
                runtime_root = pathlib.Path(args.runtime_dir)
                if runtime_root not in jail.parents or ".jailer" not in jail.parts:
                    raise RuntimeError("Owned jail cleanup escaped its namespace")
                if jail.exists():
                    shutil.rmtree(jail)
            path = pathlib.Path(args.runtime_dir) / ("volume-" + root)
            if path.is_dir():
                shutil.rmtree(path)
            elif path.exists():
                path.unlink()
        if data_created:
            data_path = pathlib.Path(args.runtime_dir) / ("volume-" + data_id)
            data_path.unlink(missing_ok=True)
        # The caller owns the disposable receiver scope. Its immutable receipt
        # and old SQLite snapshot remain available for review after the drill.


if __name__ == "__main__":
    main()
