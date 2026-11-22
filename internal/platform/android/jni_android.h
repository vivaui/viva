// The host's calls, from the Go program's threads.

#include <jni.h>
#include <stdint.h>

/* log writes a line to the system log under the tag "viva". */
void viva_log(int priority, const char *line);

/* attach_thread makes the calling OS thread known to the JVM; it stays attached. */
int viva_attach_thread(void);

/* string_chars copies a Java string to malloc memory as modified UTF-8; free it. */
char *viva_string_chars(JNIEnv *env, jstring value);

/* The text helpers (dev.viva.host.Text), over UTF-16. */
float viva_text_width(const uint16_t *value, int length, float size, int weight, int italic, int mono);
void viva_text_metrics(float size, int weight, int italic, int mono, float *out);
int viva_text_rects(const uint16_t *value, int length, float size, int weight, int italic, int mono, int rtl,
                    const int32_t *bounds, int boundCount, float *lefts, float *rights);
void viva_text_set_family(const uint16_t *name, int length);
int viva_text_begin(int width, int height);
void viva_text_draw(const uint16_t *value, int length, float x, float baseline, float size, int weight, int italic,
                    int mono, int argb, int underline, int rtl);
int viva_text_end(void *out, int64_t capacity);

/* The host (dev.viva.host.Host). */
void viva_host_finish(void);
void viva_host_set_title(const uint16_t *title, int length);
