//go:build darwin && !ios && cgo

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>
#import <Security/SecTask.h>

#include <stdlib.h>
#include "native_darwin.h"

struct viva_device_key_operation { LAContext *context; };

static NSString *application_identifier(void) {
    SecTaskRef task = SecTaskCreateFromSelf(kCFAllocatorDefault);
    if (task == NULL) return nil;
    CFTypeRef value = SecTaskCopyValueForEntitlement(task, CFSTR("com.apple.application-identifier"), NULL);
    CFRelease(task);
    if (value == NULL) return nil;
    if (CFGetTypeID(value) != CFStringGetTypeID() || CFStringGetLength((CFStringRef)value) == 0) {
        CFRelease(value);
        return nil;
    }
    return [(NSString *)value autorelease];
}

int viva_device_key_supported(void) {
    @autoreleasepool {
        // Data Protection Keychain access requires a provisioned application
        // identity. Bare/ad-hoc binaries must not advertise an unlock method
        // that will later fail with errSecMissingEntitlement.
        if (application_identifier() == nil) return 0;
        LAContext *context = [[LAContext alloc] init];
        BOOL supported = [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:NULL];
        BOOL biometric = supported && [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:NULL]
            && context.biometryType == LABiometryTypeTouchID;
        [context release];
        return biometric ? 2 : (supported ? 1 : 0);
    }
}

viva_device_key_operation *viva_device_key_operation_create(void) {
    @autoreleasepool {
    viva_device_key_operation *operation = calloc(1, sizeof *operation);
    if (operation != NULL) {
        operation->context = [[LAContext alloc] init];
        operation->context.touchIDAuthenticationAllowableReuseDuration = 0;
        if (operation->context == nil) { free(operation); return NULL; }
    }
    return operation;
    }
}

void viva_device_key_operation_cancel(viva_device_key_operation *operation) {
    @autoreleasepool { [operation->context invalidate]; }
}

void viva_device_key_operation_release(viva_device_key_operation *operation) {
    @autoreleasepool { [operation->context release]; free(operation); }
}

void viva_device_key_bytes_release(void *bytes, size_t size) {
    if (bytes != NULL) {
        volatile unsigned char *cursor = bytes;
        for (size_t i = 0; i < size; i++) cursor[i] = 0;
        free(bytes);
    }
}

int64_t viva_device_key_operation_run(viva_device_key_operation *operation,
    const char *service, const char *account, const char *reason, int action,
    const void *input, size_t input_size, void **output, size_t *output_size) {
    *output = NULL;
    *output_size = 0;
    @autoreleasepool {
        NSString *service_name = [NSString stringWithUTF8String:service];
        NSString *account_name = [NSString stringWithUTF8String:account];
        NSString *prompt = [NSString stringWithUTF8String:reason];
        if (service_name == nil || account_name == nil || prompt == nil) return errSecParam;
        NSString *group = application_identifier();
        if (group == nil) return errSecMissingEntitlement;
        NSMutableDictionary *query = [NSMutableDictionary dictionaryWithDictionary:@{
            (id)kSecClass: (id)kSecClassGenericPassword,
            (id)kSecAttrAccessGroup: group,
            (id)kSecAttrService: service_name,
            (id)kSecAttrAccount: account_name,
            (id)kSecAttrSynchronizable: @NO,
            (id)kSecUseDataProtectionKeychain: @YES,
            (id)kSecUseAuthenticationContext: operation->context,
        }];
        if (action != 1 && prompt.length != 0) operation->context.localizedReason = prompt;
        if (action == 1) {
            CFErrorRef error = NULL;
            SecAccessControlRef access = SecAccessControlCreateWithFlags(kCFAllocatorDefault,
                kSecAttrAccessibleWhenUnlockedThisDeviceOnly, kSecAccessControlUserPresence, &error);
            if (access == NULL) {
                if (error != NULL) CFRelease(error);
                return errSecNotAvailable;
            }
            query[(id)kSecAttrAccessControl] = (id)access;
            query[(id)kSecAttrLabel] = service_name;
            CFDataRef data = CFDataCreateWithBytesNoCopy(kCFAllocatorDefault, input,
                (CFIndex)input_size, kCFAllocatorNull);
            if (data == NULL) { CFRelease(access); return errSecAllocate; }
            query[(id)kSecValueData] = (id)data;
            OSStatus status = SecItemAdd((CFDictionaryRef)query, NULL);
            CFRelease(data);
            CFRelease(access);
            return status;
        }
        if (action == 3) return SecItemDelete((CFDictionaryRef)query);
        if (action != 2) return errSecParam;
        query[(id)kSecReturnData] = @YES;
        query[(id)kSecMatchLimit] = (id)kSecMatchLimitOne;
        CFTypeRef result = NULL;
        OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
        if (status != errSecSuccess) return status;
        if (result == NULL || CFGetTypeID(result) != CFDataGetTypeID()) {
            if (result != NULL) CFRelease(result);
            return errSecDecode;
        }
        CFDataRef data = (CFDataRef)result;
        CFIndex count = CFDataGetLength(data);
        if (count <= 0 || count > 2048) { CFRelease(data); return errSecDecode; }
        void *copy = malloc((size_t)count);
        if (copy == NULL) { CFRelease(data); return errSecAllocate; }
        CFDataGetBytes(data, CFRangeMake(0, count), copy);
        CFRelease(data);
        *output = copy;
        *output_size = (size_t)count;
        return errSecSuccess;
    }
}
