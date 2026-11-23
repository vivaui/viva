#!/bin/sh
# Checks the release sizes this repository promises. Run from anywhere; the
# script builds through the same viva build path a package uses, so it measures
# the flags a release actually gets (trimpath, stripped link, fixed build ID,
# the macOS deployment target and the Windows GUI subsystem).
#
#   benchmarks/size.sh
#
# The limits are deliberately above the current numbers: they are a regression
# alarm, not a claim that every future feature fits in the same bytes.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
over_limit=0

check() {
    name=$1
    target=$2
    arch=$3
    pkg=$4
    limit=$5
    out="$tmp/$(basename "$pkg").$target"
    go run -C "$root" ./cmd/viva build --target "$target" --arch "$arch" "$pkg" -o "$out" >/dev/null
    size=$(wc -c <"$out" | tr -d ' ')
    printf '%-28s %10d B  (limit %d)\n' "$name" "$size" "$limit"
    if [ "$size" -gt "$limit" ]; then
        echo "size: $name is $size bytes, over the $limit-byte limit" >&2
        over_limit=1
    fi
}

hello_limit=$((3750 * 1024))   # 3.75 MiB, covers all three native targets
gallery_limit=$((4 * 1024 * 1024))
gallery_windows_limit=$((5 * 1024 * 1024))

check "hello darwin/arm64" darwin arm64 ./examples/guides/hello "$hello_limit"
check "hello windows/amd64" windows amd64 ./examples/guides/hello "$hello_limit"
check "hello linux/amd64" linux amd64 ./examples/guides/hello "$hello_limit"
check "showcase darwin/arm64" darwin arm64 ./cmd/showcase "$gallery_limit"
check "showcase windows/amd64" windows amd64 ./cmd/showcase "$gallery_windows_limit"

exit "$over_limit"
