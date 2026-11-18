#!/bin/bash
# Cross-compile a package's tests, run them in the test environment, print the
# verdicts. The environment has no Go; the test binary does not need it.
#
#   go-test.sh linux ./internal/pkg [test flags]     (run from a Go module)
#
# Tests that open windows need the desktop session, which the environment has,
# and a Vulkan driver: the private copy under /tmp/viva-vulkan when there is
# one, otherwise the system's (mesa-vulkan-drivers). With neither, graphics tests skip.
set -eu
here=$(dirname "$0")
. "$here/env.sh"
platform=$1 package=$2
shift 2
name=$(echo "$package" | tr '/.' '__')
out=$(mktemp -d)
case "$platform" in
linux)
	GOOS=linux GOARCH=amd64 go test -c -o "$out/t" "$package"
	"$here/upload.sh" linux "$out/t" "/home/user/gotest-$name"
	remote_env="HOME=/tmp/gotest-home SHELL=/bin/bash XDG_RUNTIME_DIR=$VIVA_LINUX_RUNTIME_DIR DISPLAY=$VIVA_LINUX_DISPLAY"
	if [ -n "$VIVA_LINUX_XAUTHORITY" ]; then
		remote_env="$remote_env XAUTHORITY=$VIVA_LINUX_XAUTHORITY"
	fi
	"$here/exec.sh" linux "chmod +x /home/user/gotest-$name; mkdir -p /tmp/gotest-home /tmp/gotest-run-$name; cd /tmp/gotest-run-$name; if [ -f /tmp/viva-vulkan/lvp-local.json ]; then export VK_ICD_FILENAMES=/tmp/viva-vulkan/lvp-local.json LD_LIBRARY_PATH=/tmp/viva-vulkan/root/usr/lib/x86_64-linux-gnu; fi; env $remote_env timeout 100 /home/user/gotest-$name -test.count=1 $* 2>&1 | tail -40"
	;;
*)
	echo "first argument is linux" >&2
	exit 2
	;;
esac
