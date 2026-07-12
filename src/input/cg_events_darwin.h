#ifndef CG_EVENTS_DARWIN_H
#define CG_EVENTS_DARWIN_H

#include <stdint.h>

// cg_type_unicode posts chars (a UTF-16 buffer of length len) as a single
// synthetic keyboard event via the macOS HID event tap. Returns 0 on
// success, -1 if the event could not be created (e.g. missing
// Accessibility permission).
int cg_type_unicode(const uint16_t *chars, int len);

#endif
