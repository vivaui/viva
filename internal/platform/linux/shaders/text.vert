#version 450

layout(push_constant) uniform TextFrame {
    vec2 viewport;
    vec2 origin;
    vec2 size;
    vec2 unused;
} frame;

layout(location = 0) out vec2 outUV;
// outLocal is where the fragment falls in the label's own box, in logical
// pixels; the fragment cuts the picture's rounded corners with it.
layout(location = 1) out vec2 outLocal;

const vec2 corners[6] = vec2[](
    vec2(0.0, 0.0),
    vec2(1.0, 0.0),
    vec2(0.0, 1.0),
    vec2(0.0, 1.0),
    vec2(1.0, 0.0),
    vec2(1.0, 1.0)
);

void main() {
    vec2 corner = corners[gl_VertexIndex];
    vec2 pixel = frame.origin + corner * frame.size;
    gl_Position = vec4(pixel / frame.viewport * 2.0 - 1.0, 0.0, 1.0);
    outUV = corner;
    outLocal = corner * frame.size;
}
