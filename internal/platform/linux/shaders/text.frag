#version 450

layout(set = 0, binding = 0) uniform sampler2D labelTexture;

// mask is (label width, label height, corner radius, fade): the same rounded-box
// mask and opacity the scene shader draws, in the label's logical pixels.
layout(push_constant) uniform Mask {
    layout(offset = 48) vec4 mask;
} m;

layout(location = 0) in vec2 inUV;
layout(location = 1) in vec2 inLocal;
layout(location = 0) out vec4 outColor;

void main() {
    vec4 texel = texture(labelTexture, inUV);
    float fade = clamp(1.0 - m.mask.w, 0.0, 1.0);
    float radius = min(m.mask.z, min(m.mask.x, m.mask.y) * 0.5);
    float coverage = 1.0;
    if (radius > 0.0) {
        vec2 centered = inLocal - m.mask.xy * 0.5;
        vec2 inner = m.mask.xy * 0.5 - radius;
        vec2 distanceVector = abs(centered) - inner;
        float distance = length(max(distanceVector, 0.0)) + min(max(distanceVector.x, distanceVector.y), 0.0) - radius;
        // inLocal is in logical pixels; one device pixel is its derivative.
        float devicePixel = max(fwidth(inLocal.x), 1e-4);
        coverage = clamp(0.5 - distance / devicePixel, 0.0, 1.0);
    }
    outColor = texel * (coverage * fade);
}
