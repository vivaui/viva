// textMode is 0 when the label texture is premultiplied color, 1 when it holds the coverage
// of ClearType text in its three color channels, to be blended with textColor per channel.
// textShape is the label's box size and its image corner radius and fade:
// xy = size, z = radius, w = fade (1-opacity).
cbuffer Frame : register(b0) { float2 viewport; uint instanceOffset; uint textMode; float4 textBounds; float4 textColor; float4 textShape; float pixelSize; float3 framePadding; };
// shade1 and shade2 are a triangle's colors at its second and third corner; color is the first's.
struct Rectangle { float4 bounds; float4 color; float4 style; float4 shade1; float4 shade2; };
StructuredBuffer<Rectangle> rectangles : register(t0);
Texture2D labelTexture : register(t1);
SamplerState labelSampler : register(s0);
struct Vertex { float4 position : SV_Position; float4 color : COLOR; float2 local : LOCAL; float2 size : SIZE; float radius : RADIUS; float2 uv : TEXCOORD; float2 frag : FRAG; float2 triA : TRIA; float2 triB : TRIB; float2 triC : TRIC; nointerpolation uint triangleMask : TRIMASK; };
// Low bits keep edgeMask+1; this bit is local to the rasterizer.
#define boundedTriangleFlag (1u << 8)
float2 corner(uint vertex) {
    const float2 corners[6] = {float2(0,0),float2(1,0),float2(0,1),float2(0,1),float2(1,0),float2(1,1)};
    return corners[vertex];
}
float edgeDistance(float2 p, float2 a, float2 b) {
    float2 edge = b - a;
    float length = max(sqrt(edge.x*edge.x + edge.y*edge.y), 1e-6);
    return (edge.x*(p.y-a.y) - edge.y*(p.x-a.x)) / length;
}
float cross2(float2 a, float2 b) { return a.x*b.y - a.y*b.x; }
float segmentDistance(float2 p, float2 a, float2 b) {
    float2 edge = b - a;
    float along = saturate(dot(p - a, edge) / max(dot(edge, edge), 1e-12));
    return length(p - a - along * edge);
}
// Move only silhouette edges out by half a device pixel. Unmarked shared edges
// keep their original supporting line, so the hardware top-left rule still
// assigns each fragment to exactly one triangle. Fragment distances and color
// interpolation stay measured against the original (unexpanded) triangle.
float2 fringeVertex(float2 previous, float2 position, float2 next, uint previousEdge, uint nextEdge, uint mask) {
    float2 before = position - previous, after = next - position;
    float area = cross2(before, after);
    if (abs(area) < 1e-8 || mask == 0) return position;
    float orientation = area > 0 ? 1 : -1;
    float h = 0.5 * pixelSize * orientation;
    float first = (mask & previousEdge) != 0 ? -h * length(before) : 0;
    float second = (mask & nextEdge) != 0 ? -h * length(after) : 0;
    return position + (first * after - second * before) / area;
}
Vertex rectangleVertex(uint vertex : SV_VertexID, uint instance : SV_InstanceID) {
    Rectangle rectangle = rectangles[instanceOffset + instance];
    float2 pixel;
    float2 local;
    float2 size = rectangle.bounds.zw;
    float radius = rectangle.style.x;
    float2 triA = 0, triB = 0, triC = 0;
    uint triangleMask = 0;
    float4 color = rectangle.color;
    if (rectangle.style.w > 0.5) {
        // Triangle slots: bounds=(p0, p1),
        // style=(p2.x, layer, p2.y, edgeMask+1), and the corner colors
        // color, shade1, shade2.
        // Isolated regular faces use one triangle; meshes and slivers use a quad.
        const float2 points[3] = {
            rectangle.bounds.xy,
            rectangle.bounds.zw,
            rectangle.style.xz,
        };
        uint index = vertex < 3 ? vertex : 0;
        uint edgeMask = (uint)(rectangle.style.w + 0.5) - 1;
        uint previous = (index + 2) % 3;
        float2 ab = points[1] - points[0], ac = points[2] - points[0];
        float2 bc = points[2] - points[1];
        float area = cross2(ab, ac);
        float longest2 = max(dot(ab, ab), max(dot(ac, ac), dot(bc, bc)));
        bool valid = all(isfinite(points[0])) && all(isfinite(points[1])) && all(isfinite(points[2])) &&
            isfinite(area) && isfinite(longest2) && abs(area) >= 1e-8;
        // A sliver's offset lines can meet thousands of pixels away. Rasterize
        // its finite support instead. Every face with a shared edge uses this
        // path: mixing hardware and analytic edge ownership cracks at fractional
        // scales because hardware snaps geometry to its own subpixel grid.
        // Above this aspect ratio the miter displacement is at most eight
        // device pixels; below it the quad avoids both overflow and overdraw.
        bool bounded = valid && (edgeMask != 7 || abs(area) < longest2 * 0.125);
        if (!valid) {
            pixel = 0;
        } else if (bounded) {
            float2 low = min(points[0], min(points[1], points[2])) - 0.5 * pixelSize;
            float2 high = max(points[0], max(points[1], points[2])) + 0.5 * pixelSize;
            pixel = lerp(low, high, corner(vertex));
        } else {
            pixel = fringeVertex(points[previous], points[index], points[(index + 1) % 3], 1u << previous, 1u << index, edgeMask);
        }
        // Extrapolate corner colors so interpolation stays on the original mesh.
        if (valid) {
            float2 delta = pixel - points[0];
            color = rectangle.color + cross2(delta, ac) / area * (rectangle.shade1 - rectangle.color)
                + cross2(ab, delta) / area * (rectangle.shade2 - rectangle.color);
        }
        local = 0;
        size = 0;
        radius = 0;
        triA = points[0];
        triB = points[1];
        triC = points[2];
        triangleMask = edgeMask + 1 + (bounded ? boundedTriangleFlag : 0);
    } else if (rectangle.style.z > 0) {
        // A soft shadow: bounds and style.x are the rounded rectangle that casts it,
        // style.z its blur, shade1 = (offset.x, offset.y, spread). The quad grows by
        // the most the shadow reaches, and local stays measured from the casting
        // rectangle's corner. Offset, spread and blur reach the pixel shader in triA
        // and triB, which a rectangle has no use for.
        float extent = rectangle.style.z + max(rectangle.shade1.z, 0) + max(max(rectangle.shade1.x, -rectangle.shade1.x), max(rectangle.shade1.y, -rectangle.shade1.y));
        pixel = rectangle.bounds.xy - extent + corner(vertex) * (rectangle.bounds.zw + 2 * extent);
        local = pixel - rectangle.bounds.xy;
        triA = rectangle.shade1.xy;
        triB = float2(rectangle.shade1.z, rectangle.style.z);
    } else {
        pixel = rectangle.bounds.xy + corner(vertex) * rectangle.bounds.zw;
        local = corner(vertex) * rectangle.bounds.zw;
    }
    Vertex output;
    output.position = float4(pixel / viewport * float2(2,-2) + float2(-1,1),0,1);
    output.color = color;
    output.local = local;
    output.size = size;
    output.radius = radius;
    output.uv = corner(vertex);
    output.frag = pixel;
    output.triA = triA;
    output.triB = triB;
    output.triC = triC;
    output.triangleMask = triangleMask;
    return output;
}
// Logical pixels per device pixel at this fragment. Coordinates here are logical, so
// an edge feathered over a fixed logical band would be that many device pixels wide on
// a scaled display; antialiasing one device pixel wide keeps thin strokes and corners as
// crisp as they are on the other platforms, at any scale. It is computed before any
// branch, since derivatives are undefined in divergent flow.
float devicePixel(float2 position) {
    return max(fwidth(position).x, 1e-4);
}
// Signed distance from p to a rounded box centred on the origin.
float roundedBoxDistance(float2 p, float2 halfSize, float radius) {
    float2 q = abs(p) - halfSize + radius;
    return length(max(q, 0)) + min(max(q.x, q.y), 0) - radius;
}
// A soft shadow. The shadow's shape is the casting rectangle moved by the offset and
// grown by the spread (the corner radius grows with it, a sharp corner staying sharp).
// Its edge is spread over a band of blur on either side: a smoothstep of the signed
// distance, which follows a Gaussian of deviation blur / 2 (the CSS box-shadow blur) to
// within a few percent. The casting rectangle itself is cut out, so a translucent surface
// drawn over the shadow is not darkened by it.
float4 shadowColor(Vertex input, float pixel) {
    float2 offset = input.triA;
    float spread = input.triB.x;
    float blur = input.triB.y;
    float2 halfSize = max(input.size * 0.5 + spread, 0);
    float radius = input.radius > 0 ? clamp(input.radius + spread, 0, min(halfSize.x, halfSize.y)) : 0;
    float distance = roundedBoxDistance(input.local - input.size * 0.5 - offset, halfSize, radius);
    float t = saturate(0.5 - distance / (2 * blur));
    float alpha = t * t * (3 - 2 * t);
    float hole = min(input.radius, min(input.size.x, input.size.y) * 0.5);
    float inside = roundedBoxDistance(input.local - input.size * 0.5, input.size * 0.5, hole);
    alpha *= 1 - saturate(0.5 - inside / pixel);
    alpha *= input.color.a;
    return float4(input.color.rgb * alpha, alpha);
}
float4 rectangleFragment(Vertex input) : SV_Target {
    float shapePixel = devicePixel(input.local);
    if (input.triangleMask > 0) {
        // Winding-independent coverage: the signed distance to the nearest
        // outer mesh edge is feathered over one device pixel, the same band the
        // rounded-rect corners use. Shared tessellation edges stay opaque.
        float area2 = (input.triB.x-input.triA.x)*(input.triC.y-input.triA.y) -
            (input.triB.y-input.triA.y)*(input.triC.x-input.triA.x);
        float sign = area2 > 0 ? 1 : -1;
        uint edgeMask = (input.triangleMask & ~boundedTriangleFlag) - 1;
        // The actual sample, not the interpolated face: over a sliver's long
        // support the interpolant cannot say which pixel it is.
        float2 p = input.position.xy * pixelSize;
        float pixel = pixelSize;
        // Marked edges are supporting lines: only their half-pixel neighborhood
        // of the triangle itself is coverage. Without this cut a sliver paints
        // its lines across the window.
        float2 low = min(input.triA, min(input.triB, input.triC)) - 0.5 * pixel;
        float2 high = max(input.triA, max(input.triB, input.triC)) + 0.5 * pixel;
        if (any(p < low) || any(p > high)) {
            return float4(0, 0, 0, 0);
        }
        if ((input.triangleMask & boundedTriangleFlag) != 0) {
            float2 points[3] = {input.triA, input.triB, input.triC};
            [unroll]
            for (uint i = 0; i < 3; i++) {
                if ((edgeMask & (1u << i)) != 0) {
                    continue;
                }
                // Evaluate in device coordinates: dividing a sample by 3 first
                // can turn an exactly shared sample into a gap at 3x scale.
                float2 a = points[i] / pixel, b = points[(i + 1) % 3] / pixel;
                float direction = sign;
                // Both faces evaluate the same ordered endpoints, so rounding
                // cannot put a sample on the outside (or inside) of both.
                if (a.x > b.x || (a.x == b.x && a.y > b.y)) {
                    float2 swap = a;
                    a = b;
                    b = swap;
                    direction = -direction;
                }
                float2 edge = b - a;
                float side = direction * cross2(edge, input.position.xy - a);
                edge *= direction;
                // Screen-space top-left rule: a shared edge belongs to one face.
                bool topLeft = edge.y < 0 || (edge.y == 0 && edge.x > 0);
                if (side < 0 || (side == 0 && !topLeft)) {
                    return float4(0, 0, 0, 0);
                }
            }
        }
        float d0 = (edgeMask & 1u) != 0 ? sign * edgeDistance(p, input.triA, input.triB) : 1e6;
        float d1 = (edgeMask & 2u) != 0 ? sign * edgeDistance(p, input.triB, input.triC) : 1e6;
        float d2 = (edgeMask & 4u) != 0 ? sign * edgeDistance(p, input.triC, input.triA) : 1e6;
        float inside = min(d0, min(d1, d2));
        if (inside < 0) {
            // Acute offset-line miters must not paint beyond the actual ends.
            float s0 = (edgeMask & 1u) != 0 ? segmentDistance(p, input.triA, input.triB) : 1e6;
            float s1 = (edgeMask & 2u) != 0 ? segmentDistance(p, input.triB, input.triC) : 1e6;
            float s2 = (edgeMask & 4u) != 0 ? segmentDistance(p, input.triC, input.triA) : 1e6;
            inside = -min(s0, min(s1, s2));
        }
        float coverage = edgeMask == 0 ? 1 : saturate(0.5 + inside / pixel);
        float4 shade = saturate(input.color);
        return float4(shade.rgb*coverage*shade.a, shade.a*coverage);
    }
    if (input.triB.y > 0) {
        return shadowColor(input, shapePixel);
    }
    float radius = min(input.radius, min(input.size.x,input.size.y)*0.5);
    float coverage = 1;
    if (radius > 0) {
        float2 distanceVector = abs(input.local-input.size*0.5)-input.size*0.5+radius;
        float distance = length(max(distanceVector,0))+min(max(distanceVector.x,distanceVector.y),0)-radius;
        coverage = saturate(0.5 - distance / shapePixel);
    }
    float alpha = input.color.a*coverage;
    return float4(input.color.rgb*alpha,alpha);
}
Vertex textVertex(uint vertex : SV_VertexID) {
    Vertex output = (Vertex)0;
    float2 pixel = textBounds.xy+corner(vertex)*textBounds.zw;
    output.position = float4(pixel/viewport*float2(2,-2)+float2(-1,1),0,1);
    output.uv = corner(vertex);
    output.local = corner(vertex)*textBounds.zw;
    output.size = textShape.xy;
    output.radius = textShape.z;
    return output;
}
// Text is blended with dual-source blending: color is what is added to the target and mask
// the factor per channel that the target is kept by (target = color + target * (1 - mask)).
struct TextTarget { float4 color : SV_Target0; float4 mask : SV_Target1; };
TextTarget textFragment(Vertex input) {
    float4 texel = labelTexture.Sample(labelSampler,input.uv);
    // An image label cuts its picture's rounded corners and fades the whole
    // picture; ordinary text has a zero radius and fade and keeps the fast path.
    float radius = min(input.radius, min(input.size.x,input.size.y)*0.5);
    float rounded = 1;
    if (radius > 0) {
        float2 distanceVector = abs(input.local-input.size*0.5)-input.size*0.5+radius;
        float distance = length(max(distanceVector,0))+min(max(distanceVector.x,distanceVector.y),0)-radius;
        rounded = saturate(0.5 - distance / pixelSize);
    }
    float opacity = saturate(1 - textShape.w) * rounded;
    TextTarget output;
    if (textMode == 1) {
        float3 coverage = texel.rgb * textColor.a * opacity;
        output.color = float4(textColor.rgb * coverage, texel.a * textColor.a * opacity);
        output.mask = float4(coverage, texel.a * textColor.a * opacity);
    } else {
        output.color = texel * opacity;
        output.mask = texel.aaaa * opacity;
    }
    return output;
}
