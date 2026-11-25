//go:build darwin && !ios && cgo

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include "native_darwin.h"

int viva_systemauth_method(void) {
    @autoreleasepool {
        LAContext *context = [[[LAContext alloc] init] autorelease];
        if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:NULL]) return 0;
        if ([context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:NULL]
            && context.biometryType == LABiometryTypeTouchID) return 2;
        return 3;
    }
}

viva_auth_context viva_systemauth_new(void) {
    @autoreleasepool {
        LAContext *context = [[LAContext alloc] init];
        context.touchIDAuthenticationAllowableReuseDuration = 0;
        return context;
    }
}

void viva_systemauth_cancel(viva_auth_context ctx) {
    @autoreleasepool { [(LAContext *)ctx invalidate]; }
}

void viva_systemauth_free(viva_auth_context ctx) {
    @autoreleasepool { [(LAContext *)ctx release]; }
}

int viva_systemauth_verify(viva_auth_context ctx, const char *reason) {
    @autoreleasepool {
        LAContext *context = (LAContext *)ctx;
        NSString *prompt = [NSString stringWithUTF8String:reason];
        if (prompt.length == 0) return LAErrorInvalidContext;
        __block int result = LAErrorAuthenticationFailed;
        dispatch_semaphore_t completion = dispatch_semaphore_create(0);
        [context evaluatePolicy:LAPolicyDeviceOwnerAuthentication localizedReason:prompt reply:^(BOOL success, NSError *error) {
            result = success ? 0 : (error == nil ? LAErrorAuthenticationFailed : (int)error.code);
            dispatch_semaphore_signal(completion);
        }];
        dispatch_semaphore_wait(completion, DISPATCH_TIME_FOREVER);
        dispatch_release(completion);
        return result;
    }
}
