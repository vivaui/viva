#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <stdint.h>

extern int vivaGlobalShortcutEvent(uint32_t identifier, uint32_t kind);

static const OSType vivaHotKeySignature = 0x56697661; // 'Viva'
static EventHandlerRef vivaHotKeyHandler;
static uint32_t vivaHotKeyCount;

static OSStatus viva_hot_key_event(EventHandlerCallRef next, EventRef event, void *unused) {
    EventHotKeyID hotkey;
    OSStatus status = GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
                                        NULL, sizeof(hotkey), NULL, &hotkey);
    if (status != noErr || hotkey.signature != vivaHotKeySignature) {
        return eventNotHandledErr;
    }
    return vivaGlobalShortcutEvent(hotkey.id, GetEventKind(event)) ? noErr : eventNotHandledErr;
}

static void viva_hot_key_main(void (^work)(void)) {
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
}

int32_t viva_global_shortcut_register(uint32_t code, uint32_t modifiers, uint32_t identifier, EventHotKeyRef *reference) {
    __block OSStatus status = noErr;
    viva_hot_key_main(^{
        EventTargetRef target = GetApplicationEventTarget();
        if (vivaHotKeyHandler == NULL) {
            const EventTypeSpec types[] = {
                {kEventClassKeyboard, kEventHotKeyPressed},
                {kEventClassKeyboard, kEventHotKeyReleased},
            };
            status = InstallEventHandler(target, viva_hot_key_event, 2, types, NULL, &vivaHotKeyHandler);
            if (status != noErr) {
                return;
            }
        }
        EventHotKeyID hotkey = {vivaHotKeySignature, identifier};
        status = RegisterEventHotKey(code, modifiers, hotkey, target, 0, reference);
        if (status == noErr) {
            ++vivaHotKeyCount;
        } else if (vivaHotKeyCount == 0) {
            RemoveEventHandler(vivaHotKeyHandler);
            vivaHotKeyHandler = NULL;
        }
    });
    return status;
}

void viva_global_shortcut_unregister(EventHotKeyRef reference) {
    viva_hot_key_main(^{
        UnregisterEventHotKey(reference);
        if (--vivaHotKeyCount == 0) {
            RemoveEventHandler(vivaHotKeyHandler);
            vivaHotKeyHandler = NULL;
        }
    });
}

int32_t viva_global_shortcut_test_event(uint32_t identifier, uint32_t kind) {
    __block OSStatus status;
    viva_hot_key_main(^{
        EventRef event = NULL;
        status = CreateEvent(NULL, kEventClassKeyboard, kind, GetCurrentEventTime(), kEventAttributeNone, &event);
        if (status != noErr) {
            return;
        }
        EventHotKeyID hotkey = {vivaHotKeySignature, identifier};
        status = SetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, sizeof(hotkey), &hotkey);
        if (status == noErr) {
            status = SendEventToEventTarget(event, GetApplicationEventTarget());
        }
        ReleaseEvent(event);
    });
    return status;
}
