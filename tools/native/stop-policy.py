#!/usr/bin/env python3
"""Check per-call stop policies using a boot-only Firecracker fixture image."""
import argparse
import hashlib
import json
import pathlib
import time
import urllib.error
import urllib.request
import uuid

from native_request import response as native_response


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--controller", required=True)
    parser.add_argument("--key-file", required=True)
    parser.add_argument("--image", default="fc-alpine")
    args = parser.parse_args()
    key = pathlib.Path(args.key_file).read_text().strip()
    identity = "stop-policy-" + str(uuid.uuid4()) + "-g1"
    token = 0

    def request(method, path, payload=None):
        nonlocal token
        body = json.dumps(payload).encode() if payload is not None else None
        headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
        if method != "GET":
            token += 1
            headers["X-Compute-Command"] = json.dumps({
                "subject": identity, "generation": 1, "token": token,
                "key": str(uuid.uuid4()), "fingerprint": hashlib.sha256(body or b"").hexdigest(),
            })
        req = urllib.request.Request(args.controller + path, data=body, headers=headers, method=method)
        return native_response(req, 30)

    path = "/api/envs/" + identity

    def poll(want):
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            status, result = request("GET", path)
            if status == 200 and result["data"]["vms"][0]["state"] == want:
                return result
            time.sleep(0.2)
        raise RuntimeError("guest failed to reach " + want)

    try:
        status, result = request("POST", "/api/envs", {
            "id": identity, "owner": "native-stop-policy", "lifetime": 0,
            "vms": [{"image": args.image, "engine": "firecracker", "cpu": 1,
                     "mem": 256, "disk": 1024, "isolated_network": True}],
        })
        assert status == 202, result
        poll("running")
        began = time.monotonic()
        status, result = request("POST", path + "/stop", {"grace_seconds": 1, "graceful_only": True})
        assert status == 409 and result["failure"]["code"] == "graceful_timeout", result
        graceful_elapsed = time.monotonic() - began
        poll("running")
        began = time.monotonic()
        status, result = request("POST", path + "/stop", {"force": True})
        assert status == 200, result
        force_elapsed = time.monotonic() - began
        poll("stopped")
        status, result = request("POST", path + "/start", {})
        assert status == 200, result
        poll("running")
        began = time.monotonic()
        status, result = request("POST", path + "/stop", {"grace_seconds": 1})
        assert status == 200, result
        bounded_elapsed = time.monotonic() - began
        assert bounded_elapsed >= 0.9, bounded_elapsed
        poll("stopped")
        print(json.dumps({"result": "PASS", "identity": identity,
                          "graceful_only_seconds": round(graceful_elapsed, 3),
                          "force_seconds": round(force_elapsed, 3),
                          "bounded_grace_seconds": round(bounded_elapsed, 3)}))
    finally:
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            request("DELETE", path)
            status, _ = request("GET", path)
            if status == 404:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("could not confirm cleanup of " + identity)


if __name__ == "__main__":
    main()
