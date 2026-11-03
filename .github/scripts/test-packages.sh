#!/bin/sh
# Prints the Go packages one CI job tests, one import path per line.
#
#   .github/scripts/test-packages.sh darwin
#       The packages that carry a darwin-only file, and so can only be tested on
#       macOS, where the cgo build reaches Cocoa.
#
#   .github/scripts/test-packages.sh linux [SHARD TOTAL]
#       Every other package, split into TOTAL shards, printing shard SHARD
#       (1-based). A package that is plain Go and has no darwin file produces the
#       same result on every operating system, so it runs on Linux: testing it on
#       macOS as well would only spend macOS minutes twice.
#
# Both lists are computed from the tree instead of being written out here, so a
# new package is picked up by the job that can test it without anyone editing a
# list. A package that carries both a darwin file and files for another system
# appears in both lists: each build is checked where it can be checked.
#
# COVERAGE is the union of the two lists minus the two packages below, which
# render or drive a window like an application and cannot run on a bare runner;
# they have never run in CI, and docs/conformance.md is where a real window is
# verified.
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo"

# The directories holding darwin-only code: a file named *_darwin*.go, or any
# file whose build constraint names darwin (some are called *_native.go). A
# constraint naming !darwin counts too: the package then has an operating-system
# split, and over-reporting only moves a package to the stricter runner.
darwin_dirs() {
    {
        git ls-files '*_darwin*.go'
        git grep -l -E '^//go:build .*darwin|^// \+build .*darwin' -- '*.go' || true
    } | sed 's|/[^/]*$||' | sort -u
}

split() {
    total=$1
    shard=$2
    awk -v total="$total" -v shard="$shard" '((NR - 1) % total) + 1 == shard'
}

case "${1:-}" in
darwin)
    DIRS=$(darwin_dirs)
    export DIRS
    go list -f '{{.Dir}} {{.ImportPath}}' ./... | awk -v root="$repo/" '
        BEGIN { n = split(ENVIRON["DIRS"], a, "\n"); for (i = 1; i <= n; i++) want[a[i]] = 1 }
        { if (substr($1, length(root) + 1) in want) print $2 }'
    ;;
linux)
    DIRS=$(darwin_dirs)
    export DIRS
    go list -f '{{.Dir}} {{.ImportPath}}' ./... | awk -v root="$repo/" '
        BEGIN { n = split(ENVIRON["DIRS"], a, "\n"); for (i = 1; i <= n; i++) want[a[i]] = 1 }
        {
            if (substr($1, length(root) + 1) in want) next
            # Needs a Vulkan renderer and Pango: only a runner that installs both
            # can run it, and vector-coverage.yml is that runner.
            if ($2 == "github.com/vivaui/viva/headless") next
            # Drives X11 directly.
            if ($2 == "github.com/vivaui/viva/internal/platform/linux") next
            print $2
        }' | split "${3:-1}" "${2:-1}"
    ;;
*)
    echo "usage: $0 darwin | linux [SHARD TOTAL]" >&2
    exit 2
    ;;
esac
