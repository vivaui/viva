#version 450

layout(location = 0) in vec4 inColor;
layout(location = 1) in vec2 inLocal;
layout(location = 2) in vec2 inSize;
layout(location = 3) in float inRadius;
layout(location = 4) flat in vec2 inCoverageC;
layout(location = 5) in vec2 inTriangleA;
layout(location = 6) in vec2 inTriangleB;
layout(location = 7) in vec2 inTriangleC;
layout(location = 8) flat in uint inTriangleMask;

layout(push_constant) uniform Frame {
    vec2 viewport;
    float pixelSize;
    float unused;
    // Where the content starts in the target, in device pixels.
    vec2 origin;
} frame;

layout(location = 0) out vec4 outColor;

// Low bits keep edgeMask+1; this bit is local to the rasterizer.
#define boundedTriangleFlag (1u << 8)
#define strokeQuadEdgeMask (1u << 3)
#define coverageSourceFlag (1u << 4)

float edgeDistance(vec2 point, vec2 from, vec2 to) {
    vec2 edge = to - from;
    float edgeLength = max(length(edge), 1e-6);
    return (edge.x * (point.y - from.y) - edge.y * (point.x - from.x)) / edgeLength;
}
float cross2(vec2 a, vec2 b) { return a.x*b.y - a.y*b.x; }

float segmentDistance(vec2 p, vec2 a, vec2 b) {
    vec2 edge = b - a;
    float along = clamp(dot(p - a, edge) / max(dot(edge, edge), 1e-12), 0.0, 1.0);
    return length(p - a - along * edge);
}

// Logical pixels per device pixel at this fragment. Edges feathered over a
// fixed logical band smear across several device pixels on a scaled display;
// one device pixel keeps thin strokes and corners crisp at any scale.
float devicePixel(vec2 position) {
    return max(fwidth(position).x, 1e-4);
}

// Signed distance from p to a rounded box centred on the origin.
float roundedBoxDistance(vec2 p, vec2 halfSize, float radius) {
    vec2 q = abs(p) - halfSize + radius;
    return length(max(q, vec2(0.0))) + min(max(q.x, q.y), 0.0) - radius;
}

// A soft shadow. The shadow's shape is the casting rectangle moved by the offset
// and grown by the spread (the corner radius grows with it, a sharp corner
// staying sharp). Its edge is spread over a band of blur on either side: a
// smoothstep of the signed distance, which follows a Gaussian of deviation
// blur / 2 (the CSS box-shadow blur) to within a few percent. The casting
// rectangle itself is cut out, so a translucent surface drawn over the shadow is
// not darkened by it.
vec4 shadowColor(float pixel) {
    vec2 offset = inTriangleA;
    float spread = inTriangleB.x;
    float blur = inTriangleB.y;
    vec2 halfSize = max(inSize * 0.5 + spread, vec2(0.0));
    float radius = inRadius > 0.0 ? clamp(inRadius + spread, 0.0, min(halfSize.x, halfSize.y)) : 0.0;
    float distance = roundedBoxDistance(inLocal - inSize * 0.5 - offset, halfSize, radius);
    float t = clamp(0.5 - distance / (2.0 * blur), 0.0, 1.0);
    float alpha = t * t * (3.0 - 2.0 * t);
    float hole = min(inRadius, min(inSize.x, inSize.y) * 0.5);
    float inside = roundedBoxDistance(inLocal - inSize * 0.5, inSize * 0.5, hole);
    alpha *= 1.0 - clamp(0.5 - inside / pixel, 0.0, 1.0);
    alpha *= inColor.a;
    return vec4(inColor.rgb * alpha, alpha);
}

void main() {
    if (inTriangleMask > 0) {
        // Winding-independent coverage: the signed distance to the nearest
        // outer mesh edge is feathered over one device pixel, the same band the
        // rounded-rect corners use. Shared tessellation edges stay opaque.
        float area = (inTriangleB.x - inTriangleA.x) * (inTriangleC.y - inTriangleA.y) -
            (inTriangleB.y - inTriangleA.y) * (inTriangleC.x - inTriangleA.x);
        float orientation = area > 0.0 ? 1.0 : -1.0;
        uint edgeFlags = (inTriangleMask & ~boundedTriangleFlag) - 1u;
        uint edgeMask = edgeFlags & 7u;
        // The actual sample in the content's device pixels, as D3D and Metal
        // take it: the interpolated face would put a sample that lies exactly on
        // a shared edge on either side by rounding.
        vec2 device = gl_FragCoord.xy - frame.origin;
        vec2 p = device * frame.pixelSize;
        float pixel = frame.pixelSize;
        // Marked edges are supporting lines: only their half-pixel neighborhood
        // of the triangle itself is coverage. Without this cut a sliver paints
        // its lines across the window.
        vec2 low = min(inTriangleA, min(inTriangleB, inTriangleC)) - 0.5 * pixel;
        vec2 high = max(inTriangleA, max(inTriangleB, inTriangleC)) + 0.5 * pixel;
        if (any(lessThan(p, low)) || any(greaterThan(p, high))) {
            outColor = vec4(0.0);
            return;
        }
        if ((inTriangleMask & boundedTriangleFlag) != 0u) {
            vec2 points[3] = vec2[](inTriangleA, inTriangleB, inTriangleC);
            for (uint i = 0u; i < 3u; i++) {
                if ((edgeMask & (1u << i)) != 0u) {
                    continue;
                }
                // Evaluate in device coordinates: dividing a sample by 3 first
                // can turn an exactly shared sample into a gap at 3x scale.
                vec2 a = points[i] / pixel, b = points[(i + 1u) % 3u] / pixel;
                float direction = orientation;
                // Both faces evaluate the same ordered endpoints, so rounding
                // cannot put a sample on the outside (or inside) of both.
                if (a.x > b.x || (a.x == b.x && a.y > b.y)) {
                    vec2 swap = a;
                    a = b;
                    b = swap;
                    direction = -direction;
                }
                vec2 edge = b - a;
                float side = direction * cross2(edge, device - a);
                edge *= direction;
                // Screen-space top-left rule: a shared edge belongs to one face.
                bool topLeft = edge.y < 0.0 || (edge.y == 0.0 && edge.x > 0.0);
                if (side < 0.0 || (side == 0.0 && !topLeft)) {
                    outColor = vec4(0.0);
                    return;
                }
            }
        }
        // The clipped face owns the sample. Its original outline determines
        // the feathering, even when clipping removed every silhouette edge.
        vec2 coverageA = inTriangleA, coverageB = inTriangleB, coverageC = inTriangleC;
        uint coverageFlags = edgeFlags;
        float coverageSign = orientation;
        if ((edgeFlags & coverageSourceFlag) != 0u) {
            coverageA = inLocal;
            coverageB = inSize;
            coverageC = inCoverageC;
            coverageFlags = uint(inRadius + 0.5);
            coverageSign = cross2(coverageB - coverageA, coverageC - coverageA) > 0.0 ? 1.0 : -1.0;
        }
        uint coverageMask = coverageFlags & 7u;
        float distance0 = (coverageMask & 1u) != 0u ? coverageSign * edgeDistance(p, coverageA, coverageB) : 1e6;
        float distance1 = (coverageMask & 2u) != 0u ? coverageSign * edgeDistance(p, coverageB, coverageC) : 1e6;
        float distance2 = (coverageMask & 4u) != 0u ? coverageSign * edgeDistance(p, coverageC, coverageA) : 1e6;
        float inside = min(distance0, min(distance1, distance2));
        bool strokeQuad = (coverageFlags & strokeQuadEdgeMask) != 0u;
        vec2 oppositeStart, oppositeEnd;
        if (strokeQuad) {
            // The shared diagonal decides ownership, but coverage depends on both long edges.
            bool firstHalf = (coverageMask & 1u) != 0u;
            vec2 from = firstHalf ? coverageA : coverageC;
            vec2 to = firstHalf ? coverageB : coverageA;
            oppositeStart = firstHalf ? coverageC : coverageB;
            oppositeEnd = oppositeStart + (to - from);
            float primary = firstHalf ? distance0 : distance2;
            inside = min(inside, coverageSign * edgeDistance(oppositeStart, from, to) - primary);
        }
        if (inside < 0.0) {
            // Use the visible edge segments for the outward fringe. A clipped
            // stroke's original edges may extend beyond the cut.
            bool clippedStroke = (edgeFlags & coverageSourceFlag) != 0u;
            uint fringeMask = clippedStroke ? edgeMask : coverageMask;
            float s0 = (fringeMask & 1u) != 0u ? segmentDistance(p, inTriangleA, inTriangleB) : 1e6;
            float s1 = (fringeMask & 2u) != 0u ? segmentDistance(p, inTriangleB, inTriangleC) : 1e6;
            float s2 = (fringeMask & 4u) != 0u ? segmentDistance(p, inTriangleC, inTriangleA) : 1e6;
            float opposite = strokeQuad && !clippedStroke ? segmentDistance(p, oppositeStart, oppositeEnd) : 1e6;
            inside = -min(min(s0, s1), min(s2, opposite));
        }
        float coverage = coverageMask == 0u ? 1.0 : clamp(0.5 + inside / pixel, 0.0, 1.0);
        vec4 shade = clamp(inColor, 0.0, 1.0);
        float alpha = shade.a * coverage;
        outColor = vec4(shade.rgb * alpha, alpha);
        return;
    }
    if (inTriangleB.y > 0.0) {
        outColor = shadowColor(devicePixel(inLocal));
        return;
    }
    float radius = min(inRadius, min(inSize.x, inSize.y) * 0.5);
    float coverage = 1.0;
    if (radius > 0.0) {
        vec2 distanceVector = abs(inLocal - inSize * 0.5) - inSize * 0.5 + radius;
        float distance = length(max(distanceVector, vec2(0.0))) + min(max(distanceVector.x, distanceVector.y), 0.0) - radius;
        coverage = clamp(0.5 - distance / devicePixel(inLocal), 0.0, 1.0);
    }
    float alpha = inColor.a * coverage;
    outColor = vec4(inColor.rgb * alpha, alpha);
}
