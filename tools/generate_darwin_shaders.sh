#!/bin/sh

set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
shader_dir="$root/internal/platform/darwin/shaders"
air="$root/build/scene.air"

# A Metal library records the macOS it was built for and refuses to load on
# anything older, so build for the oldest macOS Viva supports rather than the
# host's default. Keep this in step with tools/manifest.MacOSFloor: the shader
# target test in internal/platform/darwin fails when the shipped library asks
# for a newer system.
minimum_macos=12.0

mkdir -p "$root/build"

xcrun --sdk macosx metal \
    -mmacosx-version-min="$minimum_macos" \
    -c "$shader_dir/scene.metal" \
    -o "$air"
xcrun --sdk macosx metallib \
    "$air" \
    -o "$shader_dir/scene.metallib"

unlink "$air"

# iOS has two: a Metal library records the platform it was built for, and the
# simulator's is not the device's (shader_ios.go picks the one it runs on).
minimum_ios=16.0

xcrun --sdk iphoneos metal \
    -mios-version-min="$minimum_ios" \
    -c "$shader_dir/scene.metal" \
    -o "$air"
xcrun --sdk iphoneos metallib \
    "$air" \
    -o "$shader_dir/scene_ios.metallib"
unlink "$air"

xcrun --sdk iphonesimulator metal \
    -mios-simulator-version-min="$minimum_ios" \
    -c "$shader_dir/scene.metal" \
    -o "$air"
xcrun --sdk iphonesimulator metallib \
    "$air" \
    -o "$shader_dir/scene_ios_simulator.metallib"
unlink "$air"
