#version 300 es
precision highp float;

uniform vec2 uViewport;
// Top-left, top-right, bottom-left, bottom-right, in logical pixels.
uniform vec4 uCorners[4];
uniform vec4 uTint[4];
// uLabel is the label's box in logical pixels (origin x, y, size x, y); uMask
// is its corner radius and how much of it is transparent (1-opacity).
uniform vec4 uLabel;
uniform vec2 uMask;

out vec2 vUV;
out vec4 vTint;
out vec2 vLocal;
out vec2 vSize;
out float vRadius;
out float vFade;

const vec2 uv[6] = vec2[](
    vec2(0.0, 0.0),
    vec2(1.0, 0.0),
    vec2(0.0, 1.0),
    vec2(0.0, 1.0),
    vec2(1.0, 0.0),
    vec2(1.0, 1.0)
);
const int corner[6] = int[](0, 1, 2, 2, 1, 3);

void main() {
    int index = corner[gl_VertexID];
    vec2 pixel = uCorners[index].xy;
    gl_Position = vec4(
        pixel.x / uViewport.x * 2.0 - 1.0,
        1.0 - pixel.y / uViewport.y * 2.0,
        0.0, 1.0);
    vUV = uv[gl_VertexID];
    vTint = uTint[index];
    vLocal = pixel - uLabel.xy;
    vSize = uLabel.zw;
    vRadius = uMask.x;
    vFade = uMask.y;
}
