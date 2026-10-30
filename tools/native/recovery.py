#!/usr/bin/env python3
"""Validate a complete stopped workspace checkpoint and clone-only restoration."""
import argparse
import hashlib
import json
import os
import pathlib
import subprocess
import tempfile
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
    args = p.parse_args()
    key = pathlib.Path(args.key_file).read_text().strip()
    run = str(uuid.uuid4())
    source = "recovery-source-" + run + "-g1"
    target = "recovery-target-" + run + "-g1"
    point = "recovery-" + run + "-g1"
    data = "data-" + run + "-g1"
    volumes, envs = [data], []
    token = 0
    marker = "checkpoint-" + run

    def call(method, path, body=None, subject=None, allowed=(200, 201, 202)):
        nonlocal token
        raw = json.dumps(body).encode() if body is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        if method != "GET":
            token += 1
            headers["X-Compute-Command"] = json.dumps({"subject": subject, "generation": 1, "token": token,
                "key": str(uuid.uuid4()), "fingerprint": hashlib.sha256(method.encode() + path.encode() + (raw or b"")).hexdigest()})
        req = urllib.request.Request(args.controller + path, data=raw, method=method, headers=headers)
        status, value = native_response(req, 120)
        if allowed is not None and status not in allowed:
            raise RuntimeError("%s %s: %d %s" % (method, path, status, json.dumps(value)))
        return status, value

    def root_path(identity):
        return pathlib.Path(args.runtime_dir) / ("volume-" + identity) / "rootfs.ext4"

    def data_path(identity):
        if args.backend == "zvol":
            return pathlib.Path("/dev/zvol") / args.dataset / ("volume-" + identity)
        return pathlib.Path(args.runtime_dir) / ("volume-" + identity)

    def write_root(identity, content):
        with tempfile.NamedTemporaryFile(mode="w") as probe:
            probe.write(content)
            probe.flush()
            subprocess.run(["debugfs", "-w", "-R", "write " + probe.name + " /checkpoint-validation", str(root_path(identity))], check=True, capture_output=True)

    def read_root(identity):
        return subprocess.run(["debugfs", "-R", "cat /checkpoint-validation", str(root_path(identity))], check=True, capture_output=True, text=True).stdout

    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        _, hosts = call("GET", "/api/hosts")
        if any(host["state"] == "online" and host["storage"] == args.backend
               and host.get("storage_features", {}).get("workspace_snapshots")
               and args.image in host["images"] for host in hosts["data"]):
            break
        time.sleep(0.5)
    else:
        raise RuntimeError("No checkpoint-qualified host became ready")

    try:
        call("POST", "/api/volumes", {"id": data, "generation": 1, "size_mib": 16, "retain": False}, data)
        envs.append(source)
        _, result = call("POST", "/api/envs", {"id": source, "owner": "native-recovery", "lifetime": 0,
            "vms": [{"image": args.image, "engine": "firecracker", "cpu": 1, "mem": 256, "disk": 1024,
                      "durable_root": True, "root_retain": False, "data_volume_ids": [data],
                      "initial_stopped": True, "isolated_network": True}]}, source)
        root = result["data"]["vms"][0]["options"]["root_volume_id"]
        volumes.append(root)
        write_root(root, marker)
        with data_path(data).open("r+b") as stream:
            stream.seek(4096)
            stream.write(marker.encode())
            stream.flush()
            os.fsync(stream.fileno())
        _, result = call("POST", "/api/envs/" + source + "/recovery-points", {"id": point}, source)
        manifest = result["data"]
        assert manifest["state"] == "ready" and len(manifest["disks"]) == 2, manifest
        assert {d["role"] for d in manifest["disks"]} == {"root", "data"}, manifest
        call("POST", "/api/envs/" + source + "/start", {}, source)
        call("POST", "/api/envs/" + source + "/stop", {"force": True}, source)
        envs.append(target)
        _, result = call("POST", "/api/envs/" + target + "/restore", {"point_id": point, "owner": "native-restored"}, target)
        vm = result["data"]["vms"][0]
        restored_root = vm["options"]["root_volume_id"]
        restored_data = vm["options"]["data_volumes"][0]["id"]
        volumes.extend([restored_root, restored_data])
        assert vm["state"] == "stopped" and restored_root != root and restored_data != data, vm
        assert marker in read_root(restored_root), "root checkpoint was not restored"
        with data_path(restored_data).open("rb") as stream:
            stream.seek(4096)
            assert stream.read(len(marker)) == marker.encode(), "data checkpoint was not restored"
        assert marker in read_root(root), "restore modified its source"
        call("POST", "/api/envs/" + target + "/start", {}, target)
        call("POST", "/api/envs/" + target + "/stop", {"force": True}, target)
        status, result = call("DELETE", "/api/recovery-points/" + point, subject=point, allowed=(409,))
        assert result["failure"]["code"] == "snapshot_dependency", result
        print(json.dumps({"result": "PASS", "run": run, "checks": ["complete root/data manifest", "writer fence released",
              "new stopped VM", "new disk identities", "root/data contents", "source preserved", "restored boot", "ancestry guard"]}))
    finally:
        end = time.monotonic() + 120
        while time.monotonic() < end:
            for identity in envs:
                call("DELETE", "/api/envs/" + identity, subject=identity, allowed=None)
            for identity in reversed(volumes):
                call("PATCH", "/api/volumes/" + identity, {"retain": False}, identity, allowed=None)
                call("DELETE", "/api/volumes/" + identity, subject=identity, allowed=None)
            call("DELETE", "/api/recovery-points/" + point, subject=point, allowed=None)
            remaining = []
            for identity in volumes:
                status, value = call("GET", "/api/volumes/" + identity, allowed=None)
                if status != 404 and value.get("data", {}).get("state") != "deleted":
                    remaining.append(identity)
            if not remaining:
                print(json.dumps({"cleanup": "PASS", "run": run}))
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("recovery cleanup incomplete: " + json.dumps(remaining))


if __name__ == "__main__":
    main()
