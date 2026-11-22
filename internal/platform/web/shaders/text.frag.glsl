#version 300 es
precision highp float;

uniform sampler2D uTexture;

in vec2 vUV;
in vec4 vTint;
in vec2 vLocal;
in vec2 vSize;
in float vRadius;
in float vFade;
out vec4 outColor;

void main() {
    // Label textures carry premultiplied BGRA bytes in RGBA channels.
    vec4 texel = texture(uTexture, vUV).bgra;
    // vTint is straight RGBA; premultiply it before multiplying the texel.
    vec4 tint = vec4(vTint.rgb * vTint.a, vTint.a);
    float coverage = 1.0;
    float radius = min(vRadius, min(vSize.x, vSize.y) * 0.5);
    if (radius > 0.0) {
        vec2 centered = vLocal - vSize * 0.5;
        vec2 inner = vSize * 0.5 - radius;
        vec2 distanceVector = abs(centered) - inner;
        float distance = length(max(distanceVector, 0.0)) + min(max(distanceVector.x, distanceVector.y), 0.0) - radius;
        // vLocal is in logical pixels; one device pixel is its derivative.
        float devicePixel = max(fwidth(vLocal.x), 1e-4);
        coverage = clamp(0.5 - distance / devicePixel, 0.0, 1.0);
    }
    float opacity = clamp(1.0 - vFade, 0.0, 1.0);
    outColor = texel * tint * coverage * opacity;
}
