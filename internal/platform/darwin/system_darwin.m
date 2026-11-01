#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>

#include <stdlib.h>
#include <string.h>

#include "system_darwin.h"

static const size_t viva_max_clipboard_bytes = 64u << 20;

static char *viva_copy_utf8(NSString *value) {
    if (value == nil) {
        return NULL;
    }
    const char *utf8 = value.UTF8String;
    if (utf8 == NULL) {
        return NULL;
    }
    return strdup(utf8);
}

static void viva_set_error(char **error_message, NSString *message) {
    if (error_message == NULL) {
        return;
    }
    *error_message = viva_copy_utf8(message ?: @"unknown macOS system error");
}

static NSString *viva_string(const char *value) {
    if (value == NULL) {
        return @"";
    }
    return [NSString stringWithUTF8String:value];
}

static void viva_on_main_sync(dispatch_block_t operation) {
    if (NSThread.isMainThread) {
        operation();
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), operation);
}

// viva_run_panel shows a native panel modally and then makes the window that was
// key before it key again. The panel leaves the application without a key window
// on some systems, and the first click afterwards goes to activating one instead
// of to the control under the pointer.
static NSModalResponse viva_run_panel(NSSavePanel *panel) {
    NSWindow *previous = NSApp.keyWindow;
    NSModalResponse response = [panel runModal];
    if (previous != nil && previous.visible) {
        [previous makeKeyAndOrderFront:nil];
    }
    return response;
}

static NSArray<NSString *> *viva_extensions(
    const char *const *extensions,
    size_t extension_count,
    NSString **error
) {
    if (extension_count == 0) {
        return nil;
    }
    NSMutableArray<NSString *> *values = [NSMutableArray arrayWithCapacity:extension_count];
    for (size_t index = 0; index < extension_count; index++) {
        NSString *value = viva_string(extensions[index]);
        if (value == nil) {
            if (error != NULL) {
                *error = [NSString stringWithFormat:@"file extension %zu is not valid UTF-8", index];
            }
            return nil;
        }
        [values addObject:value];
    }
    return values;
}

int viva_system_clipboard_text(char **text, char **error_message) {
    if (text == NULL) {
        viva_set_error(error_message, @"clipboard output is missing");
        return -1;
    }
    *text = NULL;
    __block int status = 0;
    __block char *result = NULL;
    __block char *failure = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSPasteboard *pasteboard = NSPasteboard.generalPasteboard;
            NSString *value = [pasteboard stringForType:NSPasteboardTypeString];
            if (value == nil) {
                result = strdup("");
            } else if ([value lengthOfBytesUsingEncoding:NSUTF8StringEncoding] > viva_max_clipboard_bytes) {
                failure = viva_copy_utf8(@"clipboard text exceeds 64 MiB");
                status = -1;
            } else {
                result = viva_copy_utf8(value);
            }
            if (status == 0 && result == NULL) {
                failure = viva_copy_utf8(@"failed to allocate clipboard text");
                status = -1;
            }
        }
    });
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
        return status;
    }
    *text = result;
    return 0;
}

int viva_system_set_clipboard_text(const char *text, char **error_message) {
    __block int status = 0;
    __block char *failure = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSString *value = viva_string(text);
            if (value == nil) {
                failure = viva_copy_utf8(@"clipboard text is not valid UTF-8");
                status = -1;
                return;
            }
            NSPasteboard *pasteboard = NSPasteboard.generalPasteboard;
            [pasteboard clearContents];
            if (![pasteboard setString:value forType:NSPasteboardTypeString]) {
                failure = viva_copy_utf8(@"AppKit refused clipboard text");
                status = -1;
            }
        }
    });
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
    }
    return status;
}

static NSPasteboard *viva_pasteboard_named(const char *name) {
    if (name == NULL) {
        return NSPasteboard.generalPasteboard;
    }
    return [NSPasteboard pasteboardWithName:viva_string(name)];
}

static NSArray<NSPasteboardType> *viva_image_types(void) {
    return @[NSPasteboardTypePNG, NSPasteboardTypeTIFF];
}

int viva_system_clipboard_has_image(const char *pasteboard) {
    __block int has = 0;
    dispatch_block_t operation = ^{
        @autoreleasepool {
            has = [viva_pasteboard_named(pasteboard) availableTypeFromArray:viva_image_types()] != nil;
        }
    };
    if (pasteboard == NULL) {
        viva_on_main_sync(operation);
    } else {
        operation();
    }
    return has;
}

int viva_system_clipboard_image_png(const char *pasteboard, size_t max_png, size_t max_raw, uint8_t **data, size_t *length, char **error_message) {
    if (data == NULL || length == NULL) {
        viva_set_error(error_message, @"clipboard image output is missing");
        return -1;
    }
    *data = NULL;
    *length = 0;
    __block int status = 0;
    __block uint8_t *result = NULL;
    __block size_t result_length = 0;
    __block char *failure = NULL;
    dispatch_block_t operation = ^{
        @autoreleasepool {
            NSPasteboard *board = viva_pasteboard_named(pasteboard);
            NSData *png = [board dataForType:NSPasteboardTypePNG];
            if (png == nil) {
                NSData *tiff = [board dataForType:NSPasteboardTypeTIFF];
                if (tiff == nil) {
                    status = 1;
                    return;
                }
                if (tiff.length > max_raw) {
                    status = 2;
                    return;
                }
                NSBitmapImageRep *representation = [NSBitmapImageRep imageRepWithData:tiff];
                if (representation == nil) {
                    failure = viva_copy_utf8(@"clipboard TIFF could not be decoded");
                    status = -1;
                    return;
                }
                png = [representation representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
                if (png == nil) {
                    failure = viva_copy_utf8(@"clipboard image could not be converted to PNG");
                    status = -1;
                    return;
                }
            }
            if (png.length == 0) {
                status = 1;
                return;
            }
            if (png.length > max_png) {
                status = 2;
                return;
            }
            result = malloc(png.length);
            if (result == NULL) {
                failure = viva_copy_utf8(@"failed to allocate clipboard image");
                status = -1;
                return;
            }
            memcpy(result, png.bytes, png.length);
            result_length = png.length;
        }
    };
    if (pasteboard == NULL) {
        viva_on_main_sync(operation);
    } else {
        operation();
    }
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
        return status;
    }
    *data = result;
    *length = result_length;
    return 0;
}

int viva_system_pasteboard_put_image(const char *pasteboard, const uint8_t *png, size_t length, int as_tiff) {
    @autoreleasepool {
        NSPasteboard *board = viva_pasteboard_named(pasteboard);
        NSData *bytes = [NSData dataWithBytes:png length:length];
        [board clearContents];
        if (!as_tiff) {
            return [board setData:bytes forType:NSPasteboardTypePNG] ? 0 : -1;
        }
        NSBitmapImageRep *representation = [NSBitmapImageRep imageRepWithData:bytes];
        NSData *tiff = representation.TIFFRepresentation;
        return tiff != nil && [board setData:tiff forType:NSPasteboardTypeTIFF] ? 0 : -1;
    }
}

static NSCursor *viva_cursor(uint8_t shape, NSString **error) {
    switch (shape) {
        case 0:
            return NSCursor.arrowCursor;
        case 1:
            return NSCursor.IBeamCursor;
        case 2:
            return NSCursor.pointingHandCursor;
        case 3:
            return NSCursor.crosshairCursor;
        case 4:
            if (error != NULL) {
                *error = @"macOS does not expose a public wait cursor";
            }
            return nil;
        case 5:
            return NSCursor.resizeLeftRightCursor;
        case 6:
            return NSCursor.resizeUpDownCursor;
        case 7:
            if (@available(macOS 15.0, *)) {
                return [NSCursor frameResizeCursorFromPosition:NSCursorFrameResizePositionTopLeft
                                                 inDirections:NSCursorFrameResizeDirectionsAll];
            }
            if (error != NULL) {
                *error = @"diagonal resize cursors require macOS 15 or newer";
            }
            return nil;
        case 8:
            if (@available(macOS 15.0, *)) {
                return [NSCursor frameResizeCursorFromPosition:NSCursorFrameResizePositionTopRight
                                                 inDirections:NSCursorFrameResizeDirectionsAll];
            }
            if (error != NULL) {
                *error = @"diagonal resize cursors require macOS 15 or newer";
            }
            return nil;
        case 9:
            return NSCursor.operationNotAllowedCursor;
        default:
            if (error != NULL) {
                *error = [NSString stringWithFormat:@"invalid cursor shape %u", shape];
            }
            return nil;
    }
}

int viva_system_set_cursor(uint8_t shape, char **error_message) {
    __block int status = 0;
    __block char *failure = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSString *error = nil;
            NSCursor *cursor = viva_cursor(shape, &error);
            if (cursor == nil) {
                failure = viva_copy_utf8(error);
                status = -1;
                return;
            }
            [cursor set];
            for (NSWindow *window in NSApp.windows) {
                NSView *view = window.contentView;
                if (view == nil) {
                    continue;
                }
                // A Viva view remembers the cursor and re-applies it whenever
                // AppKit rebuilds its cursor rectangles, which it does after any
                // layout change; a rectangle added once would be thrown away then.
                if ([view respondsToSelector:@selector(vivaSetCursor:)]) {
                    [(id)view performSelector:@selector(vivaSetCursor:) withObject:cursor];
                } else {
                    [view discardCursorRects];
                    [view addCursorRect:view.bounds cursor:cursor];
                }
            }
        }
    });
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
    }
    return status;
}

int viva_system_open_files(
    const char *title,
    const char *const *extensions,
    size_t extension_count,
    int allow_multiple,
    int choose_directories,
    viva_system_string_array *paths,
    char **error_message
) {
    if (paths == NULL) {
        viva_set_error(error_message, @"open file output is missing");
        return -1;
    }
    paths->items = NULL;
    paths->count = 0;
    __block int status = 0;
    __block char *failure = NULL;
    __block char **items = NULL;
    __block size_t count = 0;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSString *extensionError = nil;
            NSArray<NSString *> *allowed = viva_extensions(extensions, extension_count, &extensionError);
            if (extension_count > 0 && allowed == nil) {
                failure = viva_copy_utf8(extensionError);
                status = -1;
                return;
            }
            NSString *panelTitle = viva_string(title);
            if (panelTitle == nil) {
                failure = viva_copy_utf8(@"open file title is not valid UTF-8");
                status = -1;
                return;
            }
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            panel.canChooseFiles = choose_directories == 0;
            panel.canChooseDirectories = choose_directories != 0;
            panel.allowsMultipleSelection = allow_multiple != 0;
            panel.resolvesAliases = YES;
            if (panelTitle.length > 0) {
                panel.title = panelTitle;
            }
            if (allowed.count > 0) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
                panel.allowedFileTypes = allowed;
#pragma clang diagnostic pop
                panel.allowsOtherFileTypes = NO;
            }
            NSModalResponse response = viva_run_panel(panel);
            if (response == NSModalResponseCancel) {
                status = 1;
                return;
            }
            if (response != NSModalResponseOK) {
                failure = viva_copy_utf8(@"open file panel returned an unexpected result");
                status = -1;
                return;
            }
            NSArray<NSURL *> *urls = panel.URLs;
            count = urls.count;
            if (count == 0) {
                failure = viva_copy_utf8(@"open panel returned no paths");
                status = -1;
                return;
            }
            items = calloc(count, sizeof(char *));
            if (items == NULL) {
                failure = viva_copy_utf8(@"failed to allocate open file results");
                status = -1;
                return;
            }
            for (size_t index = 0; index < count; index++) {
                NSURL *url = urls[index];
                if (!url.isFileURL || url.path == nil) {
                    failure = viva_copy_utf8(@"open file panel returned a non-file URL");
                    status = -1;
                    return;
                }
                items[index] = viva_copy_utf8(url.path);
                if (items[index] == NULL) {
                    failure = viva_copy_utf8(@"failed to allocate an open file path");
                    status = -1;
                    return;
                }
            }
        }
    });
    if (status != 0) {
        if (items != NULL) {
            viva_system_string_array partial = {items, count};
            viva_system_free_string_array(partial);
        }
        if (status < 0) {
            if (error_message != NULL) {
                *error_message = failure;
            } else {
                free(failure);
            }
        }
        return status;
    }
    paths->items = items;
    paths->count = count;
    return 0;
}

int viva_system_save_file(
    const char *title,
    const char *suggested_name,
    const char *default_extension,
    const char *const *extensions,
    size_t extension_count,
    char **path,
    char **error_message
) {
    if (path == NULL) {
        viva_set_error(error_message, @"save file output is missing");
        return -1;
    }
    *path = NULL;
    __block int status = 0;
    __block char *failure = NULL;
    __block char *result = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSString *extensionError = nil;
            NSArray<NSString *> *allowed = viva_extensions(extensions, extension_count, &extensionError);
            NSString *panelTitle = viva_string(title);
            NSString *name = viva_string(suggested_name);
            NSString *defaultType = viva_string(default_extension);
            if ((extension_count > 0 && allowed == nil) || panelTitle == nil || name == nil || defaultType == nil) {
                failure = viva_copy_utf8(extensionError ?: @"save file options are not valid UTF-8");
                status = -1;
                return;
            }
            if (defaultType.length > 0 && ![allowed containsObject:defaultType]) {
                allowed = [@[defaultType] arrayByAddingObjectsFromArray:allowed ?: @[]];
            }
            NSSavePanel *panel = [NSSavePanel savePanel];
            if (panelTitle.length > 0) {
                panel.title = panelTitle;
            }
            if (name.length > 0) {
                panel.nameFieldStringValue = name;
            }
            if (allowed.count > 0) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
                panel.allowedFileTypes = allowed;
#pragma clang diagnostic pop
                panel.allowsOtherFileTypes = NO;
                panel.extensionHidden = NO;
            }
            NSModalResponse response = viva_run_panel(panel);
            if (response == NSModalResponseCancel) {
                status = 1;
                return;
            }
            if (response != NSModalResponseOK) {
                failure = viva_copy_utf8(@"save file panel returned an unexpected result");
                status = -1;
                return;
            }
            NSURL *url = panel.URL;
            if (!url.isFileURL || url.path == nil) {
                failure = viva_copy_utf8(@"save file panel returned a non-file URL");
                status = -1;
                return;
            }
            result = viva_copy_utf8(url.path);
            if (result == NULL) {
                failure = viva_copy_utf8(@"failed to allocate save file path");
                status = -1;
            }
        }
    });
    if (status != 0) {
        free(result);
        if (status < 0) {
            if (error_message != NULL) {
                *error_message = failure;
            } else {
                free(failure);
            }
        }
        return status;
    }
    *path = result;
    return 0;
}

int viva_system_reduce_motion(void) {
    __block int reduce = 0;
    viva_on_main_sync(^{
        reduce = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion ? 1 : 0;
    });
    return reduce;
}

// viva_system_preferred_languages is the user's language list, one tag per line,
// most preferred first. The caller frees it with viva_system_free.
char *viva_system_preferred_languages(void) {
    @autoreleasepool {
        NSString *joined = [NSLocale.preferredLanguages componentsJoinedByString:@"\n"];
        return viva_copy_utf8(joined == nil ? @"" : joined);
    }
}

// viva_system_dark_appearance reports whether the system is set to its dark look:
// what the application's own appearance resolves to, which follows the system
// unless the application pinned it.
// viva_appearance_override is what viva_system_set_appearance last pinned: 0 none,
// 1 light, 2 dark. While one is pinned the application's effective appearance
// is the pinned one, so the system's own setting is read from the user defaults.
static int viva_appearance_override = 0;

// viva_system_effective_dark is whether what the application draws resolves to
// dark: the system's look, or the pinned one.
int viva_system_effective_dark(void) {
    __block int dark = 0;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSAppearance *appearance = NSApplication.sharedApplication.effectiveAppearance;
            NSAppearanceName name = [appearance bestMatchFromAppearancesWithNames:@[
                NSAppearanceNameAqua, NSAppearanceNameDarkAqua,
            ]];
            dark = [name isEqualToString:NSAppearanceNameDarkAqua] ? 1 : 0;
        }
    });
    return dark;
}

int viva_system_dark_appearance(void) {
    if (viva_appearance_override == 0) {
        return viva_system_effective_dark();
    }
    __block int dark = 0;
    viva_on_main_sync(^{
        NSString *style = [NSUserDefaults.standardUserDefaults stringForKey:@"AppleInterfaceStyle"];
        dark = [style isEqualToString:@"Dark"] ? 1 : 0;
    });
    return dark;
}

// viva_system_set_appearance pins the look of everything the system draws for the
// application (its windows' buttons, menus, file dialogs, native text editing):
// 1 light, 2 dark, 0 gives it back to the system.
void viva_system_set_appearance(int mode) {
    viva_on_main_sync(^{
        viva_appearance_override = mode;
        NSAppearance *appearance = nil;
        if (mode == 1) {
            appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
        } else if (mode == 2) {
            appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }
        NSApplication.sharedApplication.appearance = appearance;
    });
}

extern void vivaSystemAppearanceChanged(void);

// VivaAppearanceObserver hears the application's effective appearance change,
// which the system does when the user switches between light and dark, or when
// the setting follows the time of day.
@interface VivaAppearanceObserver : NSObject
@end

@implementation VivaAppearanceObserver
- (void)observeValueForKeyPath:(NSString *)keyPath
                      ofObject:(id)object
                        change:(NSDictionary *)change
                       context:(void *)context {
    vivaSystemAppearanceChanged();
}
@end

// viva_system_watch_appearance starts reporting changes of the system appearance.
// Calling it again does nothing.
void viva_system_watch_appearance(void) {
    static VivaAppearanceObserver *observer = nil;
    viva_on_main_sync(^{
        if (observer != nil) {
            return;
        }
        observer = [[VivaAppearanceObserver alloc] init];
        [NSApplication.sharedApplication addObserver:observer
                                          forKeyPath:@"effectiveAppearance"
                                             options:NSKeyValueObservingOptionNew
                                             context:NULL];
    });
}

int viva_system_open_url(const char *target, char **error_message) {
    __block int status = 0;
    __block char *failure = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            NSString *value = viva_string(target);
            NSURL *url = value == nil ? nil : [NSURL URLWithString:value];
            if (url == nil) {
                failure = viva_copy_utf8(@"external URL is not valid UTF-8 or is malformed");
                status = -1;
                return;
            }
            if (![NSWorkspace.sharedWorkspace openURL:url]) {
                failure = viva_copy_utf8(@"NSWorkspace could not open the external URL");
                status = -1;
            }
        }
    });
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
    }
    return status;
}

int viva_system_scheme_handler(const char *scheme, char **error_message) {
    // LaunchServices answers from any thread, so this does not wait for the main
    // thread, which a test binary or a caller already off it may never run.
    @autoreleasepool {
        NSString *value = viva_string(scheme);
        NSURL *url = value == nil ? nil : [NSURL URLWithString:[value stringByAppendingString:@":"]];
        if (url == nil) {
            if (error_message != NULL) {
                *error_message = viva_copy_utf8(@"URL scheme is not valid UTF-8 or is malformed");
            }
            return -1;
        }
        return [NSWorkspace.sharedWorkspace URLsForApplicationsToOpenURL:url].count > 0 ? 1 : 0;
    }
}

int viva_system_play_sound(uint8_t sound, char **error_message) {
    __block int status = 0;
    __block char *failure = NULL;
    viva_on_main_sync(^{
        @autoreleasepool {
            if (sound == 0) {
                NSBeep();
                return;
            }
            NSArray<NSString *> *names = @[@"", @"Glass", @"Purr", @"Basso", @"Pop"];
            if (sound >= names.count) {
                failure = viva_copy_utf8([NSString stringWithFormat:@"invalid system sound %u", sound]);
                status = -1;
                return;
            }
            NSSound *value = [NSSound soundNamed:names[sound]];
            if (value == nil || ![value play]) {
                failure = viva_copy_utf8([NSString stringWithFormat:@"system sound %@ is unavailable", names[sound]]);
                status = -1;
            }
        }
    });
    if (status != 0) {
        if (error_message != NULL) {
            *error_message = failure;
        } else {
            free(failure);
        }
    }
    return status;
}

@interface VivaNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation VivaNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler {
    if (@available(macOS 11.0, *)) {
        completionHandler(UNNotificationPresentationOptionSound |
                          UNNotificationPresentationOptionList |
                          UNNotificationPresentationOptionBanner);
    } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        completionHandler(UNNotificationPresentationOptionSound |
                          UNNotificationPresentationOptionAlert);
#pragma clang diagnostic pop
    }
}
@end

static VivaNotificationDelegate *viva_notification_delegate(void) {
    static VivaNotificationDelegate *delegate = nil;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        delegate = [[VivaNotificationDelegate alloc] init];
    });
    return delegate;
}

static UNNotificationSettings *viva_notification_settings(UNUserNotificationCenter *center) {
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    __block UNNotificationSettings *result = nil;
    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        result = [settings retain];
        dispatch_semaphore_signal(semaphore);
    }];
    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
    dispatch_release(semaphore);
    return [result autorelease];
}

static int viva_request_notification_authorization(
    UNUserNotificationCenter *center,
    char **error_message
) {
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    __block BOOL granted = NO;
    __block NSError *failure = nil;
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL allowed, NSError *error) {
        granted = allowed;
        failure = [error retain];
        dispatch_semaphore_signal(semaphore);
    }];
    dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
    dispatch_release(semaphore);
    if (failure != nil) {
        viva_set_error(error_message, [NSString stringWithFormat:@"notification authorization failed: %@", failure.localizedDescription]);
        [failure release];
        return -1;
    }
    if (!granted) {
        viva_set_error(error_message, @"notification permission was denied");
        return -2;
    }
    return 0;
}

int viva_system_show_fatal_error(const char *title, const char *message) {
    if (!NSThread.isMainThread) {
        return -1;
    }
    @autoreleasepool {
        NSString *alertTitle = viva_string(title);
        NSString *alertMessage = viva_string(message);
        if (alertTitle == nil || alertMessage == nil) {
            return -1;
        }
        NSApplication *application = [NSApplication sharedApplication];
        [application activateIgnoringOtherApps:YES];
        NSAlert *alert = [[[NSAlert alloc] init] autorelease];
        alert.alertStyle = NSAlertStyleCritical;
        alert.messageText = alertTitle;
        alert.informativeText = alertMessage;
        [alert addButtonWithTitle:@"OK"];
        [alert runModal];
    }
    return 0;
}

int viva_system_send_notification(
    const char *title,
    const char *body,
    char **error_message
) {
    @autoreleasepool {
        NSString *bundleIdentifier = NSBundle.mainBundle.bundleIdentifier;
        if (bundleIdentifier.length == 0) {
            viva_set_error(error_message, @"notifications require an application bundle identifier");
            return -1;
        }
        NSString *notificationTitle = viva_string(title);
        NSString *notificationBody = viva_string(body);
        if (notificationTitle == nil || notificationBody == nil) {
            viva_set_error(error_message, @"notification text is not valid UTF-8");
            return -1;
        }
        UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
        center.delegate = viva_notification_delegate();
        UNNotificationSettings *settings = viva_notification_settings(center);
        switch (settings.authorizationStatus) {
            case UNAuthorizationStatusNotDetermined:
                {
                    int authorization = viva_request_notification_authorization(center, error_message);
                    if (authorization != 0) {
                        return authorization;
                    }
                }
                break;
            case UNAuthorizationStatusDenied:
                viva_set_error(error_message, @"notification permission is denied in System Settings");
                return -2;
            case UNAuthorizationStatusAuthorized:
            case UNAuthorizationStatusProvisional:
                break;
            default:
                viva_set_error(error_message, @"notification authorization status is unsupported");
                return -1;
        }

        UNMutableNotificationContent *content = [[[UNMutableNotificationContent alloc] init] autorelease];
        content.title = notificationTitle;
        content.body = notificationBody;
        content.sound = UNNotificationSound.defaultSound;
        UNNotificationRequest *request = [UNNotificationRequest
            requestWithIdentifier:NSUUID.UUID.UUIDString
                          content:content
                          trigger:nil];
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
        __block NSError *failure = nil;
        [center addNotificationRequest:request withCompletionHandler:^(NSError *error) {
            failure = [error retain];
            dispatch_semaphore_signal(semaphore);
        }];
        dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
        dispatch_release(semaphore);
        if (failure != nil) {
            viva_set_error(error_message, [NSString stringWithFormat:@"send notification: %@", failure.localizedDescription]);
            [failure release];
            return -1;
        }
        return 0;
    }
}

void viva_system_free(void *memory) {
    free(memory);
}

void viva_system_free_string_array(viva_system_string_array values) {
    if (values.items == NULL) {
        return;
    }
    for (size_t index = 0; index < values.count; index++) {
        free(values.items[index]);
    }
    free(values.items);
}
