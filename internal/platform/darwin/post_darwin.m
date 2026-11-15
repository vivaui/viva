#import <dispatch/dispatch.h>

#include "post_darwin.h"

extern void vivaApplicationDrain(void);

void viva_application_post(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        vivaApplicationDrain();
    });
}
