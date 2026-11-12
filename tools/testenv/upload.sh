#!/bin/bash
# Copy a local file into the test environment.
#
#   upload.sh linux LOCAL /home/user/NAME
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
curl -sf -X POST "$url/setup/upload" -F "file_path=$3" -F "file_data=@$2" >/dev/null
