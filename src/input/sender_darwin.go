//go:build darwin

package input

/*
#cgo LDFLAGS: -framework ApplicationServices
#include "cg_events_darwin.h"
*/
import "C"

import (
	"fmt"
	"log"
	"unicode/utf16"
	"unsafe"
)

func typeText(text string) error {
	log.Printf("typing: %q", text)

	chars := utf16.Encode([]rune(text))
	if len(chars) == 0 {
		return nil
	}

	ret := C.cg_type_unicode((*C.uint16_t)(unsafe.Pointer(&chars[0])), C.int(len(chars)))
	if ret != 0 {
		err := fmt.Errorf("cg_type_unicode failed (missing Accessibility permission?)")
		log.Printf("typing error: %v", err)
		return err
	}
	return nil
}
