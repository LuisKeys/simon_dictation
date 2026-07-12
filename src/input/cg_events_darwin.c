#include "cg_events_darwin.h"

#include <ApplicationServices/ApplicationServices.h>

int cg_type_unicode(const uint16_t *chars, int len) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);

    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, (CGKeyCode)0, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, (CGKeyCode)0, false);
    if (keyDown == NULL || keyUp == NULL) {
        if (keyDown != NULL) CFRelease(keyDown);
        if (keyUp != NULL) CFRelease(keyUp);
        if (source != NULL) CFRelease(source);
        return -1;
    }

    CGEventKeyboardSetUnicodeString(keyDown, (UniCharCount)len, (const UniChar *)chars);
    CGEventKeyboardSetUnicodeString(keyUp, (UniCharCount)len, (const UniChar *)chars);

    CGEventPost(kCGHIDEventTap, keyDown);
    CGEventPost(kCGHIDEventTap, keyUp);

    CFRelease(keyDown);
    CFRelease(keyUp);
    if (source != NULL) CFRelease(source);
    return 0;
}
