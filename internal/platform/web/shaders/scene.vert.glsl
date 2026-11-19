#version 300 es
precision highp float;
precision highp int;

layout(location = 0) in vec4 inBounds;
layout(location = 1) in vec4 inColor;
layout(location = 2) in vec4 inStyle;
layout(location = 3) in vec4 inShade1;
layout(location = 4) in vec4 inShade2;

uniform vec2 uViewport;
uniform float uPixelSize;

// Low bits keep edgeMask+1; this bit is local to the rasterizer.
#define boundedTriangleFlag (1u << 8)

out vec4 vColor;
out vec2 vLocal;
out vec2 vSize;
out float vRadius;
out vec2 vFragment;
out vec2 vTriangleA;
out vec2 vTriangleB;
out vec2 vTriangleC;
flat out uint vTriangleMask;

const vec2 corners[6] = vec2[](
    vec2(0.0, 0.0),
    vec2(1.0, 0.0),
    vec2(0.0, 1.0),
    vec2(0.0, 1.0),
    vec2(1.0, 0.0),
    vec2(1.0, 1.0)
);

float cross2(vec2 a, vec2 b) { return a.x * b.y - a.y * b.x; }
bool finite1(float v) { return !isnan(v) && !isinf(v); }
bool finite2(vec2 v) { return finite1(v.x) && finite1(v.y); }

// Expand silhouette edges only; shared edges keep their supporting line.
vec2 fringeVertex(vec2 previous, vec2 position, vec2 next, uint previousEdge, uint nextEdge, uint mask) {
    vec2 before = position - previous, after = next - position;
    float area = cross2(before, after);
    if (abs(area) < 1e-8 || mask == 0u) return position;
    float orientation = area > 0.0 ? 1.0 : -1.0;
    float h = 0.5 * uPixelSize * orientation;
    float first = (mask & previousEdge) != 0u ? -h * length(before) : 0.0;
    float second = (mask & nextEdge) != 0u ? -h * length(after) : 0.0;
    return position + (first * after - second * before) / area;
}

void main() {
    vec2 pixel;
    vec2 localPixel;
    vec2 size = inBounds.zw;
    float radius = inStyle.x;
    vec2 triangleA = vec2(0.0);
    vec2 triangleB = vec2(0.0);
    vec2 triangleC = vec2(0.0);
    uint triangleMask = 0u;
    vec4 color = inColor;
    if (inStyle.w > 0.5) {
        vec2 points[3] = vec2[](inBounds.xy, inBounds.zw, inStyle.xz);
        uint index = gl_VertexID < 3 ? uint(gl_VertexID) : 0u;
        uint edgeMask = uint(inStyle.w + 0.5) - 1u;
        uint previous = (index + 2u) % 3u;
        vec2 ab = points[1] - points[0], ac = points[2] - points[0];
        vec2 bc = points[2] - points[1];
        float area = cross2(ab, ac);
        float longest2 = max(dot(ab, ab), max(dot(ac, ac), dot(bc, bc)));
        bool valid = finite2(points[0]) && finite2(points[1]) && finite2(points[2]) &&
            finite1(area) && finite1(longest2) && abs(area) >= 1e-8;
        // A sliver's offset lines can meet thousands of pixels away. Rasterize
        // its finite support instead. Every face with a shared edge uses this
        // path: mixing hardware and analytic edge ownership cracks at fractional
        // scales because hardware snaps geometry to its own subpixel grid.
        bool bounded = valid && (edgeMask != 7u || abs(area) < longest2 * 0.125);
        if (!valid) {
            pixel = vec2(0.0);
        } else if (bounded) {
            vec2 low = min(points[0], min(points[1], points[2])) - 0.5 * uPixelSize;
            vec2 high = max(points[0], max(points[1], points[2])) + 0.5 * uPixelSize;
            pixel = mix(low, high, corners[gl_VertexID]);
        } else {
            pixel = fringeVertex(points[previous], points[index], points[(index + 1u) % 3u], 1u << previous, 1u << index, edgeMask);
        }
        // Extrapolate corner colors so interpolation stays on the original mesh.
        if (valid) {
            vec2 delta = pixel - points[0];
            color = inColor + cross2(delta, ac) / area * (inShade1 - inColor)
                + cross2(ab, delta) / area * (inShade2 - inColor);
        }
        localPixel = vec2(0.0);
        size = vec2(0.0);
        radius = 0.0;
        triangleA = points[0];
        triangleB = points[1];
        triangleC = points[2];
        triangleMask = edgeMask + 1u + (bounded ? boundedTriangleFlag : 0u);
    } else if (inStyle.z > 0.0) {
        // A soft shadow: inBounds and inStyle.x are the rounded rectangle that
        // casts it, inStyle.z its blur, inShade1 = (offset.x, offset.y, spread).
        float extent = inStyle.z + max(inShade1.z, 0.0) + max(max(inShade1.x, -inShade1.x), max(inShade1.y, -inShade1.y));
        pixel = inBounds.xy - extent + corners[gl_VertexID] * (inBounds.zw + 2.0 * extent);
        localPixel = pixel - inBounds.xy;
        triangleA = inShade1.xy;
        triangleB = vec2(inShade1.z, inStyle.z);
    } else {
        pixel = inBounds.xy + corners[gl_VertexID] * inBounds.zw;
        localPixel = corners[gl_VertexID] * inBounds.zw;
    }
    // WebGL's NDC +1 is the top of the viewport; Viva's logical origin is top-left.
    gl_Position = vec4(
        pixel.x / uViewport.x * 2.0 - 1.0,
        1.0 - pixel.y / uViewport.y * 2.0,
        0.0, 1.0);
    vColor = color;
    vLocal = localPixel;
    vSize = size;
    vRadius = radius;
    vFragment = pixel;
    vTriangleA = triangleA;
    vTriangleB = triangleB;
    vTriangleC = triangleC;
    vTriangleMask = triangleMask;
}
