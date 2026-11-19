#version 300 es
precision highp float;

uniform sampler2D uTexture;

in vec2 vUV;
in vec4 vTint;
out vec4 outColor;

void main() {
    // Label textures carry premultiplied BGRA bytes in RGBA channels.
    vec4 texel = texture(uTexture, vUV).bgra;
    // vTint is straight RGBA; premultiply it before multiplying the texel.
    vec4 tint = vec4(vTint.rgb * vTint.a, vTint.a);
    outColor = texel * tint;
}
