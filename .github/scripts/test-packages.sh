#!/bin/sh
# Prints the Go packages one CI job tests, one import path per line.
#
#   .github/scripts/test-packages.sh darwin
#       The packages that need macOS: they carry a darwin-only file, or a test
#       that branches on runtime.GOOS and so has an arm only macOS exercises.
#
#   .github/scripts/test-packages.sh linux [SHARD TOTAL]
#       The packages that run on Linux, split into TOTAL shards, printing shard
#       SHARD (1-based). A package that is plain Go builds to the same source on
#       every operating system, so testing it on macOS as well would only spend
#       macOS minutes twice.
#
# Both lists are computed from the tree instead of written out here, so a new
# package is picked up by the job that can test it without anyone editing a
# list. A package may be in both, and that is deliberate: cmd/viva carries no
# darwin build constraint, but its packaging tests skip on every other system,
# so the darwin run is the only one that builds a macOS bundle and a disk image.
#
# Two packages are in neither list. They render or drive a window the way an
# application does, so a bare runner cannot run them; they have never run in
# CI, and docs/conformance.md is where a real window is verified.
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo"

# Directories holding code that only one operating system can exercise: a file
# named for it, a build constraint naming it, or a test that branches on
# runtime.GOOS. A constraint naming !darwin counts too: the package then has an
# operating-system split, and over-reporting only moves a package to the
# stricter runner.
os_dirs() {
    os=$1
    {
        git ls-files "*_${os}*.go"
        git grep -l -E "^//go:build .*${os}|^// \+build .*${os}" -- '*.go' || true
        if [ "$os" = darwin ]; then
            git grep -l 'runtime\.GOOS' -- '*_test.go' || true
        fi
    } | sed 's|/[^/]*$||' | sort -u
}

list() {
    go list -f '{{.Dir}} {{.ImportPath}}' ./...
}

split() {
    awk -v total="$1" -v shard="$2" '((NR - 1) % total) + 1 == shard'
}

case "${1:-}" in
darwin)
    DIRS=$(os_dirs darwin)
    export DIRS
    list | awk -v root="$repo/" '
        BEGIN { n = split(ENVIRON["DIRS"], a, "\n"); for (i = 1; i <= n; i++) want[a[i]] = 1 }
        { if (substr($1, length(root) + 1) in want) print $2 }'
    ;;
linux)
    DIRS=$(os_dirs darwin)
    export DIRS
    list | awk -v root="$repo/" '
        BEGIN { n = split(ENVIRON["DIRS"], a, "\n"); for (i = 1; i <= n; i++) want[a[i]] = 1 }
        {
            # cmd/viva and the packaging tools carry no darwin build constraint,
            # so they build here too, and their tests run here as well.
            if ($2 ~ "/(cmd/viva|tools/appbundle|tools/docs)$") { print $2; next }
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
