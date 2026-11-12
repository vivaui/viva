#ifndef VIVA_LABEL_GEOMETRY_H
#define VIVA_LABEL_GEOMETRY_H

#include <math.h>
#include <stddef.h>

typedef struct {
    float pixelX, pixelY, sourceU, sourceV;
    float tint[4];
} viva_label_vertex;

static inline double viva_label_area(viva_label_vertex first, viva_label_vertex second, viva_label_vertex third) {
    return ((double)second.pixelX - first.pixelX) * ((double)third.pixelY - first.pixelY) -
           ((double)second.pixelY - first.pixelY) * ((double)third.pixelX - first.pixelX);
}

static inline void viva_label_vertices(const float *quad, const float *tint, viva_label_vertex *vertices) {
    viva_label_vertex corners[4];
    for (int index = 0; index < 4; index++) {
        corners[index] = (viva_label_vertex){quad[index * 2], quad[index * 2 + 1], index & 1, index >> 1, {0}};
        for (int channel = 0; channel < 4; channel++) {
            corners[index].tint[channel] = tint[index * 4 + channel];
        }
    }
    int order[6] = {0, 1, 2, 1, 3, 2};
    if (viva_label_area(corners[0], corners[1], corners[2]) * viva_label_area(corners[1], corners[3], corners[2]) < 0) {
        order[2] = 3;
        order[3] = 0;
    }
    for (int index = 0; index < 6; index++) {
        vertices[index] = corners[order[index]];
    }
}

static inline float viva_label_coordinate(viva_label_vertex vertex, int axis) {
    return axis == 0 ? vertex.pixelX : axis == 1 ? vertex.pixelY : vertex.sourceU;
}

static inline size_t viva_label_clip(const viva_label_vertex *input, size_t count, viva_label_vertex *output,
                                    int axis, float boundary, int greater) {
    size_t written = 0;
    if (count == 0) {
        return 0;
    }
    viva_label_vertex previous = input[count - 1];
    float before = viva_label_coordinate(previous, axis) - boundary;
    for (size_t index = 0; index < count; index++) {
        viva_label_vertex current = input[index];
        float after = viva_label_coordinate(current, axis) - boundary;
        int inside = greater ? after >= 0 : after <= 0;
        int wasInside = greater ? before >= 0 : before <= 0;
        if (inside != wasInside) {
            float fraction = before / (before - after);
            viva_label_vertex intersection = {
                previous.pixelX + (current.pixelX - previous.pixelX) * fraction,
                previous.pixelY + (current.pixelY - previous.pixelY) * fraction,
                previous.sourceU + (current.sourceU - previous.sourceU) * fraction,
                previous.sourceV + (current.sourceV - previous.sourceV) * fraction, {0}
            };
            for (int channel = 0; channel < 4; channel++) {
                intersection.tint[channel] = previous.tint[channel] + (current.tint[channel] - previous.tint[channel]) * fraction;
            }
            if (axis == 0) intersection.pixelX = boundary;
            if (axis == 1) intersection.pixelY = boundary;
            if (axis == 2) intersection.sourceU = boundary;
            output[written++] = intersection;
        }
        if (inside) {
            output[written++] = current;
        }
        previous = current;
        before = after;
    }
    return written;
}

static inline int viva_label_source_span(const float *quad, const float *tint, float left, float top, float right, float bottom,
                                         float *sourceLeft, float *sourceRight) {
    viva_label_vertex vertices[6], first[12], second[12];
    viva_label_vertices(quad, tint, vertices);
    *sourceLeft = 1;
    *sourceRight = 0;
    for (int triangle = 0; triangle < 2; triangle++) {
        for (int corner = 0; corner < 3; corner++) first[corner] = vertices[triangle * 3 + corner];
        size_t count = viva_label_clip(first, 3, second, 0, left, 1);
        count = viva_label_clip(second, count, first, 0, right, 0);
        count = viva_label_clip(first, count, second, 1, top, 1);
        count = viva_label_clip(second, count, first, 1, bottom, 0);
        for (size_t index = 0; index < count; index++) {
            *sourceLeft = fminf(*sourceLeft, first[index].sourceU);
            *sourceRight = fmaxf(*sourceRight, first[index].sourceU);
        }
    }
    return *sourceRight > *sourceLeft;
}

static inline size_t viva_label_tile_vertices(const float *quad, const float *tint, float left, float right,
                                             float textureLeft, float textureRight, viva_label_vertex *output) {
    viva_label_vertex vertices[6], first[8], second[8];
    viva_label_vertices(quad, tint, vertices);
    if (left <= 0 && right >= 1 && textureLeft == 0 && textureRight == 1) {
        for (size_t index = 0; index < 6; index++) output[index] = vertices[index];
        return 6;
    }
    size_t written = 0;
    for (int triangle = 0; triangle < 2; triangle++) {
        size_t count = viva_label_clip(vertices + triangle * 3, 3, first, 2, left, 1);
        count = viva_label_clip(first, count, second, 2, right, 0);
        for (size_t index = 0; index < count; index++) {
            second[index].sourceU = (second[index].sourceU - textureLeft) / (textureRight - textureLeft);
        }
        for (size_t index = 1; index + 1 < count; index++) {
            output[written++] = second[0];
            output[written++] = second[index];
            output[written++] = second[index + 1];
        }
    }
    return written;
}

#endif
