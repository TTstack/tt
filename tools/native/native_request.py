"""Bounded exact-stop replay for conservative native lifecycle observations."""
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


def response(request, timeout):
    """Retry only an unsettled stop, retaining its complete command identity."""
    for attempt in range(4):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as reply:
                status, value = reply.status, json.load(reply)
        except urllib.error.HTTPError as error:
            status, value = error.code, json.load(error)
        message = value.get("error") or ""
        if not (request.get_method() == "POST"
                and urllib.parse.urlsplit(request.full_url).path.endswith("/stop")
                and status >= 500
                and "has no readable identity; resources retained" in message
                and attempt < 3):
            return status, value
        print("Stop retained an uncertain process identity; replaying the same stop", file=sys.stderr)
        time.sleep(0.1)
    raise AssertionError("stop retry bound exhausted")
