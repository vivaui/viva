#!/bin/sh
# Reports the release sizes of the example applications. Run from anywhere; the
# script builds through the same viva build path a package uses, so it measures
# the flags a release actually gets (trimpath, stripped link, fixed build ID,
# the macOS deployment target and the Windows GUI subsystem).
#
#   benchmarks/size.sh
#
# It reports and does not judge: the size of a binary is a number to watch
# across changes, not a property one fixed limit can tell right from wrong.
# On GitHub Actions the table also goes to the job summary.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
summary=${GITHUB_STEP_SUMMARY:-/dev/null}

printf '| Binary | Bytes |\n| --- | ---: |\n' >>"$summary"

report() {
    name=$1
    target=$2
    arch=$3
    pkg=$4
    out="$tmp/$(basename "$pkg").$target"
    go run -C "$root" ./cmd/viva build --target "$target" --arch "$arch" "$pkg" -o "$out" >/dev/null
    size=$(wc -c <"$out" | tr -d ' ')
    printf '%-28s %10d B\n' "$name" "$size"
    printf '| %s | %d |\n' "$name" "$size" >>"$summary"
}

report "hello darwin/arm64" darwin arm64 ./examples/guides/hello
report "hello windows/amd64" windows amd64 ./examples/guides/hello
report "hello linux/amd64" linux amd64 ./examples/guides/hello
report "showcase darwin/arm64" darwin arm64 ./cmd/showcase
report "showcase windows/amd64" windows amd64 ./cmd/showcase
