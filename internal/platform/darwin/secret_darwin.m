//go:build !ios

#import <Foundation/Foundation.h>
#import <Security/Security.h>

#include <stdlib.h>
#include <string.h>

#include "secret_darwin.h"

static NSMutableDictionary *viva_secret_query(const char *service, const char *account) {
    NSMutableDictionary *query = [NSMutableDictionary dictionary];
    query[(__bridge id)kSecClass] = (__bridge id)kSecClassGenericPassword;
    query[(__bridge id)kSecAttrService] = [NSString stringWithUTF8String:service];
    query[(__bridge id)kSecAttrAccount] = [NSString stringWithUTF8String:account];
    return query;
}

int viva_secret_store(const char *service, const char *account, const unsigned char *data, size_t length) {
    @autoreleasepool {
        NSData *value = [NSData dataWithBytes:data length:length];
        NSMutableDictionary *query = viva_secret_query(service, account);
        NSMutableDictionary *add = [query mutableCopy];
        add[(__bridge id)kSecValueData] = value;
        OSStatus status = SecItemAdd((__bridge CFDictionaryRef)add, NULL);
        if (status == errSecDuplicateItem) {
            NSDictionary *update = @{(__bridge id)kSecValueData: value};
            status = SecItemUpdate((__bridge CFDictionaryRef)query, (__bridge CFDictionaryRef)update);
        }
        return (int)status;
    }
}

int viva_secret_load(const char *service, const char *account, unsigned char **data, size_t *length) {
    @autoreleasepool {
        NSMutableDictionary *query = viva_secret_query(service, account);
        query[(__bridge id)kSecReturnData] = @YES;
        query[(__bridge id)kSecMatchLimit] = (__bridge id)kSecMatchLimitOne;
        CFTypeRef result = NULL;
        OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query, &result);
        if (status != errSecSuccess) {
            return (int)status;
        }
        NSData *value = CFBridgingRelease(result);
        *length = value.length;
        *data = malloc(value.length > 0 ? value.length : 1);
        if (*data == NULL) {
            return (int)errSecAllocate;
        }
        memcpy(*data, value.bytes, value.length);
        return 0;
    }
}

int viva_secret_delete(const char *service, const char *account) {
    @autoreleasepool {
        return (int)SecItemDelete((__bridge CFDictionaryRef)viva_secret_query(service, account));
    }
}
