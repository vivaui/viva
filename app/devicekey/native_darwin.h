#include <stddef.h>
#include <stdint.h>

typedef struct viva_device_key_operation viva_device_key_operation;
int viva_device_key_supported(void);
viva_device_key_operation *viva_device_key_operation_create(void);
void viva_device_key_operation_cancel(viva_device_key_operation *operation);
void viva_device_key_operation_release(viva_device_key_operation *operation);
int64_t viva_device_key_operation_run(viva_device_key_operation *operation,
    const char *service, const char *account, const char *reason, int action,
    const void *input, size_t input_size, void **output, size_t *output_size);
void viva_device_key_bytes_release(void *bytes, size_t size);
