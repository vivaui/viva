#ifndef VIVA_WATERMARK_H
#define VIVA_WATERMARK_H

typedef struct {
 const char *text, *font_family;
 double red, green, blue, alpha;
 float font_size, font_weight, angle, tile_width, tile_height;
 float top, right, bottom, left;
} viva_watermark;
void viva_watermark_apply(void *host, viva_watermark config);
void viva_raise_watermark(void *host);
int viva_application_set_watermark(void *window, viva_watermark config);
#endif
