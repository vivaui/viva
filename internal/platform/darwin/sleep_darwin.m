#import <Foundation/Foundation.h>
#import <IOKit/pwr_mgt/IOPMLib.h>

#include <stdio.h>
#include <stdlib.h>

#include "sleep_darwin.h"

int viva_system_keep_awake(const char *reason, uint32_t *assertion, char **error_message) {
    if (assertion == NULL) {
        if (error_message != NULL) {
            *error_message = strdup("the assertion output is missing");
        }
        return -1;
    }
    CFStringRef name = CFStringCreateWithCString(
        kCFAllocatorDefault, reason != NULL ? reason : "", kCFStringEncodingUTF8);
    if (name == NULL) {
        if (error_message != NULL) {
            *error_message = strdup("the reason is not UTF-8");
        }
        return -1;
    }
    IOPMAssertionID identifier = kIOPMNullAssertionID;
    IOReturn status = IOPMAssertionCreateWithName(
        kIOPMAssertionTypePreventUserIdleSystemSleep, kIOPMAssertionLevelOn, name, &identifier);
    CFRelease(name);
    if (status != kIOReturnSuccess) {
        if (error_message != NULL) {
            char buffer[64];
            snprintf(buffer, sizeof buffer, "IOPMAssertionCreateWithName failed (0x%08x)", status);
            *error_message = strdup(buffer);
        }
        return -1;
    }
    *assertion = (uint32_t)identifier;
    return 0;
}

void viva_system_release_awake(uint32_t assertion) {
    if (assertion != kIOPMNullAssertionID) {
        IOPMAssertionRelease((IOPMAssertionID)assertion);
    }
}
