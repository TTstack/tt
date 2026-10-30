#!/usr/bin/env python3
"""Validate independent file-backed disks on an explicitly isolated Linux agent."""
import argparse
import hashlib
import json
import pathlib
import os
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

from native_request import response as native_response


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent", required=True)
    parser.add_argument("--key-file", required=True)
    parser.add_argument("--runtime-dir", required=True)
    parser.add_argument("--image", default="fc-alpine")
    parser.add_argument("--controller-mode", action="store_true")
    parser.add_argument("--backend", choices=("file", "zvol"), default="file")
    parser.add_argument("--dataset")
    args = parser.parse_args()
    key = pathlib.Path(args.key_file).read_text().strip()
    run = str(uuid.uuid4())
    root = "root-" + run + "-g1"
    data = "data-" + run + "-g1"
    root_point = "root-point-" + run + "-g1"
    data_point = "data-point-" + run + "-g1"
    restored = "restored-" + run + "-g1"
    restored_data = "restored-data-" + run + "-g1"
    token = 0
    volumes = []
    points = []
    vms = []
    marker = "persistent-volume-" + run

    def request(method, path, payload=None, subject=None, acceptable=(200, 201)):
        nonlocal token
        body = json.dumps(payload).encode() if payload is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        if method != "GET":
            token += 1
            assert subject
            headers["X-Compute-Command"] = json.dumps({
                "subject": subject, "generation": 1, "token": token, "key": str(uuid.uuid4()),
                "fingerprint": hashlib.sha256(method.encode() + path.encode() + (body or b"")).hexdigest(),
            })
        req = urllib.request.Request(args.agent + path, data=body, headers=headers, method=method)
        status, result = native_response(req, 120)
        if acceptable is not None and status not in acceptable:
            raise RuntimeError("%s %s: %d %s" % (method, path, status, json.dumps(result)))
        return status, result

    def disk_path(identity, root_disk=False):
        path = pathlib.Path(args.runtime_dir) / ("volume-" + identity)
        if args.backend == "zvol" and not root_disk:
            if not args.dataset:
                raise RuntimeError("--dataset is required for zvol validation")
            return pathlib.Path("/dev/zvol") / args.dataset / ("volume-" + identity)
        return path / "rootfs.ext4" if root_disk else path

    def debugfs(identity, command, writing=False):
        argv = ["debugfs"] + (["-w"] if writing else []) + ["-R", command, str(disk_path(identity, True))]
        return subprocess.run(argv, check=True, capture_output=True, text=True).stdout

    def create_vm(number, durable, initial_stopped=True):
        nonlocal root
        identity = "disk-vm-" + run + "-" + str(number)
        environment = "disk-env-" + run + "-" + str(number) + "-g1"
        if args.controller_mode:
            vms.append((environment, environment))
            _, result = request("POST", "/api/envs", {
                "id": environment, "owner": "native-volume-validation", "lifetime": 0,
                "vms": [{"image": args.image, "engine": "firecracker", "cpu": 1,
                         "mem": 256, "disk": 1024, "ports": [], "deny_outgoing": False,
                         "isolated_network": True, "durable_root": durable,
                         "root_volume_id": "" if durable else root,
                         "data_volume_ids": [data], "initial_stopped": initial_stopped}],
            }, environment, acceptable=(202,))
            vm = result["data"]["vms"][0]
            if durable:
                volumes.remove(root)
                root = vm["options"]["root_volume_id"]
                volumes.append(root)
            return environment, environment, vm
        vms.append((identity, environment))
        _, result = request("POST", "/api/vms", {
            "vm_id": identity, "env_id": environment, "image": args.image, "engine": "firecracker",
            "cpu": 1, "mem": 256, "disk": 1024, "ports": [], "deny_outgoing": False,
            "isolated_network": True, "root_volume_id": root, "durable_root": durable,
            "data_volume_ids": [data], "initial_stopped": initial_stopped,
        }, environment)
        return identity, environment, result["data"]["vm"]

    def vm_path(identity):
        return ("/api/envs/" if args.controller_mode else "/api/vms/") + identity

    if args.controller_mode:
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            _, response = request("GET", "/api/hosts")
            qualified = [host for host in response["data"] if host["state"] == "online"
                         and host["storage"] == args.backend
                         and host.get("storage_features", {}).get("protocol_version") == 1
                         and args.image in host["images"]]
            if qualified:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("No qualified host became ready within the validation budget")

    try:
        volumes.extend([data, root])
        request("POST", "/api/volumes", {"id": data, "generation": 1, "size_mib": 16, "retain": True}, data)
        first, first_env, vm = create_vm(1, True)
        assert vm["state"] == "stopped", vm
        with tempfile.NamedTemporaryFile(mode="w", delete=True) as probe:
            probe.write(marker)
            probe.flush()
            debugfs(root, "write " + probe.name + " /volume-validation", True)
        with disk_path(data).open("r+b") as stream:
            stream.seek(4096)
            stream.write(marker.encode())
            stream.flush()
            os.fsync(stream.fileno())
        request("POST", vm_path(first) + "/start", {}, first_env)
        request("POST", vm_path(first) + "/stop", {"force": True}, first_env)
        points.append(root_point)
        request("POST", "/api/volumes/" + root + "/snapshots", {"id": root_point}, root)
        points.append(data_point)
        request("POST", "/api/volumes/" + data + "/snapshots", {"id": data_point}, data)
        request("DELETE", vm_path(first), subject=first_env)
        assert marker in debugfs(root, "cat /volume-validation"), "VM deletion lost root contents"
        _, retained = request("GET", "/api/volumes/" + root)
        assert retained["data"]["state"] == "ready" and not retained["data"].get("attached_to"), retained
        second, second_env, vm = create_vm(2, False)
        request("POST", vm_path(second) + "/start", {}, second_env)
        request("POST", vm_path(second) + "/stop", {"force": True}, second_env)
        with disk_path(data).open("rb") as stream:
            stream.seek(4096)
            assert stream.read(len(marker)) == marker.encode(), "rebind lost data disk contents"
        request("PATCH", "/api/volumes/" + root, {"size_mib": 1280}, root)
        request("PATCH", "/api/volumes/" + data, {"size_mib": 32}, data)
        _, grown = request("GET", "/api/volumes/" + root)
        assert grown["data"]["size_mib"] == 1280, grown
        assert marker in debugfs(root, "cat /volume-validation"), "root grow lost contents"
        request("DELETE", vm_path(second), subject=second_env)
        volumes.append(restored)
        request("POST", "/api/volumes", {"id": restored, "generation": 1, "size_mib": 1024,
                                         "retain": False, "from_snapshot": root_point}, restored)
        assert marker in debugfs(restored, "cat /volume-validation"), "restore lost root contents"
        volumes.append(restored_data)
        request("POST", "/api/volumes", {"id": restored_data, "generation": 1, "size_mib": 16,
                                         "retain": False, "from_snapshot": data_point}, restored_data)
        with disk_path(restored_data).open("rb") as stream:
            stream.seek(4096)
            assert stream.read(len(marker)) == marker.encode(), "restore lost data disk contents"
        status, result = request("DELETE", "/api/volume-snapshots/" + root_point, subject=root_point,
                                 acceptable=(409,))
        assert result["failure"]["code"] == "snapshot_dependency", result
        status, result = request("DELETE", "/api/volumes/" + root, subject=root, acceptable=(409,))
        assert result["failure"]["code"] == "volume_retained", result
        print(json.dumps({"result": "PASS", "run": run, "checks": ["stopped provision", "root and data bindings",
              "boot", "retained root contents", "offline rebind", "data persistence", "root and data growth", "root and data snapshot restore",
              "checkpoint ancestry guard", "explicit retention guard"]}))
    finally:
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline:
            for identity, environment in vms:
                request("DELETE", vm_path(identity), subject=environment, acceptable=None)
            for identity in reversed(volumes):
                request("PATCH", "/api/volumes/" + identity, {"retain": False}, identity, acceptable=None)
                request("DELETE", "/api/volumes/" + identity, subject=identity, acceptable=None)
            for identity in reversed(points):
                request("DELETE", "/api/volume-snapshots/" + identity, subject=identity, acceptable=None)
            remaining = []
            for identity in volumes:
                status, result = request("GET", "/api/volumes/" + identity, acceptable=None)
                if status != 404 and result.get("data", {}).get("state") != "deleted":
                    remaining.append(identity)
            if not remaining:
                print(json.dumps({"cleanup": "PASS", "run": run}))
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("cleanup did not confirm absence: " + json.dumps(remaining))


if __name__ == "__main__":
    main()
