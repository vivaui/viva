#!/bin/bash
# Run one command in a test environment and print its output.
#
#   exec.sh linux 'bash command'
#
# The call is synchronous and the environment cuts it off after about a minute;
# start long work in the background and read its log. Output with control
# characters breaks the reply, so filter logs to printable text.
set -eu
. "$(dirname "$0")/env.sh"
case "$1" in
linux) url=$VIVA_LINUX_ENV ;;
*) echo "first argument is linux" >&2; exit 2 ;;
esac
if [ -z "$url" ]; then
	echo "set VIVA_LINUX_ENV, or put it in tools/testenv/local.sh" >&2
	exit 2
fi
python3 - "$1" "$2" "$url" <<'PY'
import json, sys, urllib.request
kind, text, url = sys.argv[1:4]
if kind == "linux":
    command = ["bash", "-c", text]
else:
    sys.exit("first argument is linux")
body = json.dumps({"command": command, "shell": False}).encode()
request = urllib.request.Request(url + "/execute", data=body, headers={"Content-Type": "application/json"})
reply = json.load(urllib.request.urlopen(request, timeout=120))
print(reply.get("output") or "", end="")
sys.exit(0 if reply.get("returncode") == 0 else 1)
PY
