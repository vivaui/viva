#!/bin/sh

set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
shader_dir="$root/internal/platform/vulkan/shaders"

glslc --target-env=vulkan1.0 -O "$shader_dir/scene.vert" -o "$shader_dir/scene.vert.spv"
glslc --target-env=vulkan1.0 -O "$shader_dir/scene.frag" -o "$shader_dir/scene.frag.spv"
glslc --target-env=vulkan1.0 -O "$shader_dir/text.vert" -o "$shader_dir/text.vert.spv"
glslc --target-env=vulkan1.0 -O "$shader_dir/text.frag" -o "$shader_dir/text.frag.spv"
glslc --target-env=vulkan1.0 -O "$shader_dir/text_lcd.frag" -o "$shader_dir/text_lcd.frag.spv"
