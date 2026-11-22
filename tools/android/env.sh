#!/bin/sh
# Source this: the Android SDK, NDK and emulator Viva's mobile work uses.
# Paths of this machine go in tools/android/local.sh (git-ignored); the defaults
# are where Homebrew's android-commandlinetools puts them.
here=$(cd "$(dirname "$0")" && pwd)
[ -f "$here/local.sh" ] && . "$here/local.sh"
: "${ANDROID_HOME:=/opt/homebrew/share/android-commandlinetools}"
export ANDROID_HOME ANDROID_SDK_ROOT=$ANDROID_HOME
: "${ANDROID_NDK_HOME:=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort | tail -1)}"
export ANDROID_NDK_HOME
export PATH="$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator:$ANDROID_HOME/cmdline-tools/latest/bin:$PATH"
# The virtual device viva run --target android-emu starts.
: "${VIVA_ANDROID_AVD:=viva-api35}"
export VIVA_ANDROID_AVD
