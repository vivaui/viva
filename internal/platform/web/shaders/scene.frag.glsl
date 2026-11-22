#version 300 es
precision highp float;
precision highp int;

in vec4 vColor;
in vec2 vLocal;
in vec2 vSize;
in float vRadius;
in vec2 vFragment;
in vec2 vTriangleA;
in vec2 vTriangleB;
in vec2 vTriangleC;
flat in uint vTriangleMask;

uniform vec2 uTargetPixels;
uniform vec2 uOrigin;
uniform float uPixelSize;

out vec4 outColor;

#define boundedTriangleFlag (1u << 8)

float edgeDistance(vec2 point, vec2 from, vec2 to) {
    vec2 edge = to - from;
    float edgeLength = max(length(edge), 1e-6);
    return (edge.x * (point.y - from.y) - edge.y * (point.x - from.x)) / edgeLength;
}
float cross2(vec2 a, vec2 b) { return a.x * b.y - a.y * b.x; }

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
    vec2 offset = vTriangleA;
    float spread = vTriangleB.x;
    float blur = vTriangleB.y;
    vec2 halfSize = max(vSize * 0.5 + spread, vec2(0.0));
    float radius = vRadius > 0.0 ? clamp(vRadius + spread, 0.0, min(halfSize.x, halfSize.y)) : 0.0;
    float distance = roundedBoxDistance(vLocal - vSize * 0.5 - offset, halfSize, radius);
    float t = clamp(0.5 - distance / (2.0 * blur), 0.0, 1.0);
    float alpha = t * t * (3.0 - 2.0 * t);
    float hole = min(vRadius, min(vSize.x, vSize.y) * 0.5);
    float inside = roundedBoxDistance(vLocal - vSize * 0.5, vSize * 0.5, hole);
    alpha *= 1.0 - clamp(0.5 - inside / pixel, 0.0, 1.0);
    alpha *= vColor.a;
    return vec4(vColor.rgb * alpha, alpha);
}

void main() {
    // WebGL's gl_FragCoord starts at the bottom-left; Viva's device space at
    // the top-left of the target.
    vec2 device = vec2(gl_FragCoord.x, uTargetPixels.y - gl_FragCoord.y) - uOrigin;
    vec2 p = device * uPixelSize;
    float pixel = uPixelSize;
    if (vTriangleMask > 0u) {
        // Winding-independent coverage: the signed distance to the nearest
        // outer mesh edge is feathered over one device pixel, the same band the
        // rounded-rect corners use. Shared tessellation edges stay opaque.
        float area = (vTriangleB.x - vTriangleA.x) * (vTriangleC.y - vTriangleA.y) -
            (vTriangleB.y - vTriangleA.y) * (vTriangleC.x - vTriangleA.x);
        float orientation = area > 0.0 ? 1.0 : -1.0;
        uint edgeMask = (vTriangleMask & ~boundedTriangleFlag) - 1u;
        // Marked edges are supporting lines: only their half-pixel neighborhood
        // of the triangle itself is coverage. Without this cut a sliver paints
        // its lines across the window.
        vec2 low = min(vTriangleA, min(vTriangleB, vTriangleC)) - 0.5 * pixel;
        vec2 high = max(vTriangleA, max(vTriangleB, vTriangleC)) + 0.5 * pixel;
        if (any(lessThan(p, low)) || any(greaterThan(p, high))) {
            outColor = vec4(0.0);
            return;
        }
        if ((vTriangleMask & boundedTriangleFlag) != 0u) {
            vec2 points[3] = vec2[](vTriangleA, vTriangleB, vTriangleC);
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
        float distance0 = (edgeMask & 1u) != 0u ? orientation * edgeDistance(p, vTriangleA, vTriangleB) : 1e6;
        float distance1 = (edgeMask & 2u) != 0u ? orientation * edgeDistance(p, vTriangleB, vTriangleC) : 1e6;
        float distance2 = (edgeMask & 4u) != 0u ? orientation * edgeDistance(p, vTriangleC, vTriangleA) : 1e6;
        float inside = min(distance0, min(distance1, distance2));
        if (inside < 0.0) {
            // Acute offset-line miters must not paint beyond the actual ends.
            float s0 = (edgeMask & 1u) != 0u ? segmentDistance(p, vTriangleA, vTriangleB) : 1e6;
            float s1 = (edgeMask & 2u) != 0u ? segmentDistance(p, vTriangleB, vTriangleC) : 1e6;
            float s2 = (edgeMask & 4u) != 0u ? segmentDistance(p, vTriangleC, vTriangleA) : 1e6;
            inside = -min(s0, min(s1, s2));
        }
        float coverage = edgeMask == 0u ? 1.0 : clamp(0.5 + inside / pixel, 0.0, 1.0);
        vec4 shade = clamp(vColor, 0.0, 1.0);
        float alpha = shade.a * coverage;
        outColor = vec4(shade.rgb * alpha, alpha);
        return;
    }
    if (vTriangleB.y > 0.0) {
        outColor = shadowColor(devicePixel(vLocal));
        return;
    }
    float radius = min(vRadius, min(vSize.x, vSize.y) * 0.5);
    float coverage = 1.0;
    if (radius > 0.0) {
        vec2 distanceVector = abs(vLocal - vSize * 0.5) - vSize * 0.5 + radius;
        float distance = length(max(distanceVector, vec2(0.0))) + min(max(distanceVector.x, distanceVector.y), 0.0) - radius;
        coverage = clamp(0.5 - distance / devicePixel(vLocal), 0.0, 1.0);
    }
    float alpha = vColor.a * coverage;
    outColor = vec4(vColor.rgb * alpha, alpha);
}
