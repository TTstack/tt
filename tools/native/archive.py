#!/usr/bin/env python3
"""Validate portable disk archives and verified content-addressed image pulls."""
import argparse
import functools
import hashlib
import http.server
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid

from native_request import response as native_response


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--controller", required=True)
    p.add_argument("--key-file", required=True)
    p.add_argument("--runtime-dir", required=True)
    p.add_argument("--backend", choices=("file", "zvol"), default="file")
    p.add_argument("--dataset")
    p.add_argument("--image", default="fc-alpine")
    p.add_argument("--engine", choices=("firecracker", "qemu"), default="firecracker")
    p.add_argument("--source-host", default="172.31.254.1")
    p.add_argument("--source-port", type=int, default=19400)
    args = p.parse_args()
    key = pathlib.Path(args.key_file).read_text().strip()
    run = str(uuid.uuid4())
    token = 0
    volumes, environments = [], []
    marker = ("portable-" + run).encode()

    def call(method, path, body=None, subject=None, source=None, allowed=(200, 201, 202), replay=None, endpoint=None):
        nonlocal token
        raw = json.dumps(body).encode() if body is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        if method != "GET":
            token += 1
            headers["X-Compute-Command"] = replay or json.dumps({"subject": subject, "generation": 1, "token": token,
                "key": str(uuid.uuid4()), "fingerprint": hashlib.sha256(method.encode() + path.encode() + (raw or b"")).hexdigest()})
        if source:
            headers["X-Archive-Source"] = source
        req = urllib.request.Request((endpoint or args.controller) + path, data=raw, method=method, headers=headers)
        status, value = native_response(req, 180)
        if allowed is not None and status not in allowed:
            raise RuntimeError("%s %s: %d %s" % (method, path, status, json.dumps(value)))
        return status, value, headers.get("X-Compute-Command")

    class Source(http.server.SimpleHTTPRequestHandler):
        def log_message(self, *_):
            pass

    def disk_path(identity, root=False):
        if args.backend == "zvol" and not (root and args.engine == "firecracker"):
            return pathlib.Path("/dev/zvol") / args.dataset / ("volume-" + identity)
        path = pathlib.Path(args.runtime_dir) / ("volume-" + identity)
        return path / "rootfs.ext4" if root and args.engine == "firecracker" else path

    def create_env(identity, image, root=None):
        environments.append(identity)
        _, value, _ = call("POST", "/api/envs", {"id": identity, "owner": "native-archive", "lifetime": 0,
            "vms": [{"image": image, "engine": args.engine, "cpu": 1, "mem": 256, "disk": 1024,
                      "durable_root": True, "root_retain": False, "root_volume_id": root or "",
                      "initial_stopped": True, "isolated_network": True}]}, identity)
        vm = value["data"]["vms"][0]
        if not root:
            volumes.append(vm["options"]["root_volume_id"])
        assert vm["state"] == "stopped", vm
        call("POST", "/api/envs/" + identity + "/start", {}, identity)
        call("POST", "/api/envs/" + identity + "/stop", {"force": True}, identity)

    with tempfile.TemporaryDirectory(prefix="tt-archive-") as directory:
        handler = functools.partial(Source, directory=directory)
        source = http.server.ThreadingHTTPServer((args.source_host, args.source_port), handler)
        threading.Thread(target=source.serve_forever, daemon=True).start()
        source_url = "http://%s:%d/" % (args.source_host, args.source_port)
        image_id = None
        try:
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                _, hosts, _ = call("GET", "/api/hosts")
                if not any(h["state"] == "online" and h["storage"] != args.backend for h in hosts["data"]) and any(h["state"] == "online" and h["storage"] == args.backend and args.image in h["images"]
                       and h.get("storage_features", {}).get("archives") for h in hosts["data"]):
                    break
                time.sleep(0.5)
            else:
                raise RuntimeError("No archive-qualified fixture host became ready")
            specs = []
            for role in ("root", "data"):
                identity = role + "-" + run + "-g1"
                spec = {"id": identity, "generation": 1, "size_mib": 1024 if role == "root" else 16,
                        "retain": False}
                if role == "root":
                    spec.update(engine=args.engine, image=args.image)
                volumes.append(identity)
                _, result, _ = call("POST", "/api/volumes", spec, identity)
                if role == "root" and args.engine == "firecracker":
                    probe = pathlib.Path(directory) / "probe"
                    probe.write_bytes(marker)
                    subprocess.run(["debugfs", "-w", "-R", "write " + str(probe) + " /portable-marker", str(disk_path(identity, True))], check=True, capture_output=True)
                if role == "data":
                    with disk_path(identity).open("r+b") as stream:
                        stream.seek(4096)
                        stream.write(marker)
                        stream.flush()
                        os.fsync(stream.fileno())
                archive = pathlib.Path(directory) / (role + ".gz")
                req = urllib.request.Request(args.controller + "/api/volumes/" + identity + "/archive", headers={"Authorization": "Bearer " + key})
                with urllib.request.urlopen(req, timeout=180) as response, archive.open("wb") as out:
                    shutil.copyfileobj(response, out, 1 << 20)
                digest = hashlib.file_digest(archive.open("rb"), "sha256").hexdigest()
                pull = {"sha256": digest, "bytes": archive.stat().st_size, "expanded_mib": spec["size_mib"] + 80}
                new_id = role + "-copy-" + run + "-g1"
                copy_spec = dict(spec, id=new_id)
                volumes.append(new_id)
                body = {"pull": dict(pull, id=new_id), "spec": copy_spec}
                _, imported, replay = call("POST", "/api/volumes/" + new_id + "/import", body, new_id, source_url + role + ".gz")
                assert imported["data"]["state"] == "ready", imported
                _, repeated, _ = call("POST", "/api/volumes/" + new_id + "/import", body, new_id, source_url + role + ".gz?refresh=1", replay=replay)
                assert repeated["data"]["id"] == new_id, repeated
                if role == "root" and args.engine == "firecracker":
                    out = subprocess.run(["debugfs", "-R", "cat /portable-marker", str(disk_path(new_id, True))], check=True, capture_output=True).stdout
                    assert marker in out, "portable root contents changed"
                if role == "data":
                    with disk_path(new_id).open("rb") as stream:
                        stream.seek(4096)
                        assert stream.read(len(marker)) == marker, "portable data contents changed"
                if role == "root":
                    create_env("archive-boot-" + run + "-g1", args.image, new_id)
                    image_id = "sha256-" + digest + "-g1"
                    call("POST", "/api/images/" + image_id + "/ensure", {"pull": dict(pull, id=image_id)}, image_id, source_url + role + ".gz")
                    deadline = time.monotonic() + 20
                    while time.monotonic() < deadline:
                        _, hosts, _ = call("GET", "/api/hosts")
                        if any(image_id in host["images"] for host in hosts["data"] if host["state"] == "online"):
                            break
                        time.sleep(0.5)
                    else:
                        raise RuntimeError("Verified image was not advertised")
                    create_env("image-boot-" + run + "-g1", image_id)
                specs.append((role, spec, pull))
            # Corrupted bytes must retain the admitted intent and must never publish ready.
            role, spec, pull = specs[-1]
            corrupt = pathlib.Path(directory) / "corrupt.gz"
            raw = bytearray((pathlib.Path(directory) / "data.gz").read_bytes())
            raw[len(raw) // 2] ^= 1
            corrupt.write_bytes(raw)
            bad = "corrupt-" + run + "-g1"
            volumes.append(bad)
            status, refusal, _ = call("POST", "/api/volumes/" + bad + "/import", {"pull": dict(pull, id=bad), "spec": dict(spec, id=bad,retain=True)}, bad, source_url + "corrupt.gz", allowed=None)
            assert status == 503 and refusal["failure"]["class"] == "unknown", refusal
            _, retained, _ = call("GET", "/api/volumes/" + bad)
            assert retained["data"]["state"] == "creating", retained
            print(json.dumps({"result": "PASS", "run": run, "backend": args.backend, "engine": args.engine,
                "cache_image": image_id, "checks": ["portable root/data", "digest and expansion envelope", "replay with refreshed grant", "restored boot", "verified image pull and boot", "corruption retains unpublished intent", "unpublished retained import abort"]}))
        finally:
            cleanup_errors = []
            for environment in reversed(environments):
                try:
                    call("DELETE", "/api/envs/" + environment, subject=environment)
                except Exception as error:
                    cleanup_errors.append(str(error))
            for identity in reversed(volumes):
                try:
                    status, observed, _ = call("GET", "/api/volumes/" + identity, allowed=None)
                    if status == 404:
                        continue
                    if identity.startswith("corrupt-") and observed["data"]["state"] == "creating":
                        call("POST", "/api/volumes/" + identity + "/import/abort", {}, identity)
                        continue
                    if observed["data"].get("retain"):
                        call("PATCH", "/api/volumes/" + identity, {"retain": False}, identity)
                    call("DELETE", "/api/volumes/" + identity, subject=identity)
                except Exception as error:
                    cleanup_errors.append(str(error))
            if image_id and not cleanup_errors:
                _, hosts, _ = call("GET", "/api/hosts")
                for host in hosts["data"]:
                    if host["state"] == "online" and host["storage"] == args.backend:
                        call("DELETE", "/api/images/" + image_id + "/cache", subject=image_id, endpoint="http://" + host["addr"])
            if cleanup_errors:
                raise RuntimeError("Owned fixture cleanup requires inspection: " + json.dumps(cleanup_errors))
            source.shutdown()


if __name__ == "__main__":
    main()
