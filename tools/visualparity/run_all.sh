#!/bin/bash
# Run the visual parity harness on this Mac and on the Linux test machine (set
# VIVA_LINUX_ENV; see tools/testenv/env.sh), and collect the results in
# $OUT/{mac,linux} (default /tmp/parity-*), then compare them.
#
#   tools/visualparity/run_all.sh [mac|linux ...]      (from the repository root)
#
# Remote files are named with $TAG (default visualparity-$USER) and removed afterwards.
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"
T=tools/testenv
. $T/env.sh
TAG=${TAG:-visualparity-$(id -un)}
OUT=${OUT:-/tmp/parity}
platforms=("$@")
[ ${#platforms[@]} -eq 0 ] && platforms=(mac linux)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for platform in "${platforms[@]}"; do
	case $platform in
	mac)
		rm -rf "$OUT-mac"
		go run -tags vivahead ./tools/visualparity run -out "$OUT-mac" -scales 1,1.5,2 2>&1 | grep -v CoreText
		;;
	linux)
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$tmp/vp" ./tools/visualparity
		$T/upload.sh linux "$tmp/vp" "/home/user/$TAG"
		cat >"$tmp/run.sh" <<EOS
#!/bin/bash
chmod +x /home/user/$TAG
rm -rf /tmp/$TAG-out; mkdir -p /tmp/$TAG-home /tmp/$TAG-run; cd /tmp/$TAG-run
export HOME=/tmp/$TAG-home SHELL=/bin/bash DISPLAY=:1 XAUTHORITY=/home/user/.Xauthority
if [ -f /tmp/viva-vulkan/lvp-local.json ]; then export VK_ICD_FILENAMES=/tmp/viva-vulkan/lvp-local.json LD_LIBRARY_PATH=/tmp/viva-vulkan/root/usr/lib/x86_64-linux-gnu; fi
timeout 600 /home/user/$TAG run -out /tmp/$TAG-out -scales 1,1.5,2 > /tmp/$TAG.log 2>&1
echo exit=\$? >> /tmp/$TAG.log
EOS
		$T/upload.sh linux "$tmp/run.sh" "/home/user/$TAG-run.sh"
		$T/exec.sh linux "chmod +x /home/user/$TAG-run.sh; setsid nohup /home/user/$TAG-run.sh >/dev/null 2>&1 </dev/null &"
		for _ in $(seq 60); do sleep 3; $T/exec.sh linux "grep -q '^exit=' /tmp/$TAG.log" && break; done
		$T/exec.sh linux "tail -2 /tmp/$TAG.log; cd /tmp && tar czf /tmp/$TAG.tgz $TAG-out"
		rm -rf "$OUT-linux"; mkdir -p "$OUT-linux"
		curl -sf -X POST "$VIVA_LINUX_ENV/file" -F "file_path=/tmp/$TAG.tgz" -o "$tmp/linux.tgz"
		tar xzf "$tmp/linux.tgz" -C "$OUT-linux" --strip-components=1
		$T/exec.sh linux "rm -rf /home/user/$TAG /home/user/$TAG-run.sh /tmp/$TAG-out /tmp/$TAG-home /tmp/$TAG-run /tmp/$TAG.tgz /tmp/$TAG.log"
		;;
	*)
		echo "unknown platform $platform" >&2
		exit 2
		;;
	esac
done
echo "results in $OUT-{mac,linux}; compare with: go run ./tools/visualparity compare -out $OUT-diff $OUT-mac $OUT-linux"
