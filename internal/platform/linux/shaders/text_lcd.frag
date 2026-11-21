#version 450

// LCD text: the label texture holds the coverage of each color channel (red, green and blue)
// and the target is blended per channel with dual-source blending. outColor is what is added
// to the target, outMask the factor, per channel, by which the target is kept:
//   target = outColor + target * (1 - outMask).

layout(set = 0, binding = 0) uniform sampler2D labelTexture;

layout(push_constant) uniform Tint {
    layout(offset = 32) vec4 color;
    // mask is (label width, label height, corner radius, fade).
    layout(offset = 48) vec4 mask;
} tint;

layout(location = 0) in vec2 inUV;
layout(location = 1) in vec2 inLocal;
layout(location = 0, index = 0) out vec4 outColor;
layout(location = 0, index = 1) out vec4 outMask;

void main() {
    vec4 texel = texture(labelTexture, inUV);
    float fade = clamp(1.0 - tint.mask.w, 0.0, 1.0);
    float radius = min(tint.mask.z, min(tint.mask.x, tint.mask.y) * 0.5);
    float rounded = 1.0;
    if (radius > 0.0) {
        vec2 centered = inLocal - tint.mask.xy * 0.5;
        vec2 inner = tint.mask.xy * 0.5 - radius;
        vec2 distanceVector = abs(centered) - inner;
        float distance = length(max(distanceVector, 0.0)) + min(max(distanceVector.x, distanceVector.y), 0.0) - radius;
        float devicePixel = max(fwidth(inLocal.x), 1e-4);
        rounded = clamp(0.5 - distance / devicePixel, 0.0, 1.0);
    }
    float opacity = fade * rounded * tint.color.a;
    vec3 coverage = texel.rgb * opacity;
    outColor = vec4(tint.color.rgb * coverage, texel.a * opacity);
    outMask = vec4(coverage, texel.a * opacity);
}
