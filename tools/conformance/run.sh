#!/bin/bash
# Build viva-conformance for a platform, run it there, and print its report.
#
#   tools/conformance/run.sh mac        run on this Mac (opens real windows)
#   tools/conformance/run.sh linux      run in the Linux test container
#   tools/conformance/run.sh all        both, one after another
#
# Windows conformance runs by hand on a real Windows machine; see
# docs/conformance.md.
#
# The container is a machine you provide: set VIVA_LINUX_ENV to its address (or
# put it in tools/testenv/local.sh). Exit status is 1 if any platform reported a
# FAIL or could not be reached.
set -u
root=$(cd "$(dirname "$0")/../.." && pwd)
. "$root/tools/testenv/env.sh"
linux_env=${VIVA_LINUX_ENV:-}
export no_proxy="${no_proxy:-},localhost,127.0.0.1"
out=$(mktemp -d)
status=0

# execute URL JSON: run a command through a test environment's /execute endpoint.
execute() {
	python3 - "$1" "$2" <<'PY'
import json, sys, urllib.request
request = urllib.request.Request(sys.argv[1] + "/execute", data=sys.argv[2].encode(), headers={"Content-Type": "application/json"})
reply = json.load(urllib.request.urlopen(request, timeout=300))
print(reply.get("output") or "", end="")
sys.exit(0 if reply.get("returncode") == 0 else 1)
PY
}

upload() { curl -sf -X POST "$1/setup/upload" -F "file_path=$2" -F "file_data=@$3" >/dev/null; }

run_mac() {
	(cd "$root" && go build -o "$out/mac" ./cmd/viva-conformance) && (cd "$out" && ./mac)
}

run_linux() {
	if [ -z "$linux_env" ]; then
		echo "set VIVA_LINUX_ENV to a Linux test environment" >&2
		return 1
	fi
	(cd "$root" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$out/linux" ./cmd/viva-conformance) || return 1
	upload "$linux_env" /home/user/viva-conformance "$out/linux" || return 1
	# The desktop's display comes from VIVA_LINUX_DISPLAY (GNOME on X at :0 on the
	# current machine). Its GPU is software Vulkan (lavapipe). The session bus
	# address changes with each login; the standard per-user socket is used, with
	# the desktop shell's environment as a fallback.
	remote_env="HOME=/home/user DISPLAY=$VIVA_LINUX_DISPLAY XDG_RUNTIME_DIR=$VIVA_LINUX_RUNTIME_DIR"
	if [ -n "$VIVA_LINUX_XAUTHORITY" ]; then
		remote_env="$remote_env XAUTHORITY=$VIVA_LINUX_XAUTHORITY"
	fi
	remote_command="chmod +x /home/user/viva-conformance
bus=/run/user/\$(id -u)/bus
if [ -S \"\$bus\" ]; then export DBUS_SESSION_BUS_ADDRESS=unix:path=\$bus; else pid=\$(pgrep -u user 'gnome-shell|plasmashell' | head -1); if [ -r /proc/\$pid/environ ]; then export \$(xargs -0 -n1 -a /proc/\$pid/environ | grep ^DBUS_SESSION_BUS_ADDRESS=); fi; fi
if [ -f /tmp/viva-vulkan/lvp-local.json ]; then export VK_ICD_FILENAMES=/tmp/viva-vulkan/lvp-local.json LD_LIBRARY_PATH=/tmp/viva-vulkan/root/usr/lib/x86_64-linux-gnu; fi
cd /tmp && env $remote_env timeout 50 /home/user/viva-conformance"
	payload=$(python3 -c 'import json,sys; print(json.dumps({"command":["bash","-c",sys.argv[1]],"shell":False}))' "$remote_command")
	execute "$linux_env" "$payload"
}

run() {
	echo "== $1"
	local report
	report=$(run_"$1" 2>&1)
	echo "$report"
	if [ -z "$report" ] || echo "$report" | grep -q '^FAIL'; then
		status=1
	fi
}

case "${1:-}" in
mac | linux) run "$1" ;;
all) run mac; run linux ;;
*) sed -n '2,13p' "$0"; exit 2 ;;
esac
exit $status
