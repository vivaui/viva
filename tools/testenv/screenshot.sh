#!/bin/bash
# Save a 1920x1080 PNG of the test environment's desktop.   screenshot.sh linux OUT.png
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
curl -sf -m 30 -o "$2" "$url/screenshot"
