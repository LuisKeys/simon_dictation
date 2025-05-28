package vtt

/*
#cgo CFLAGS: -I${SRCDIR} -I/home/lucho/projects/ai/whisper.cpp/build/install/include
#cgo LDFLAGS: -L${SRCDIR} -L/home/lucho/projects/ai/whisper.cpp/build/install/lib -lwhisper_wrapper -lwhisper -lggml -lggml-base -lggml-cpu -lstdc++ -lm -pthread -lgomp

#include <stdlib.h>
#include "whisper_wrapper.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type WhisperModel struct {
	ctx *C.whisper_context_t
}

// NewWhisperModel loads the WhisperModel file at path (e.g. "small.bin")
func NewWhisperModel(WhisperModelPath string) *WhisperModel {
	cpath := C.CString(WhisperModelPath)
	defer C.free(unsafe.Pointer(cpath))
	ctx := C.ww_init(cpath)
	return &WhisperModel{ctx: ctx}
}

// Transcribe takes raw mono-16kHz PCM samples and returns the text.
func (m *WhisperModel) Transcribe(pcm []float32, lang string) (string, error) {
	ptr := (*C.float)(unsafe.Pointer(&pcm[0]))
	clen := C.int(len(pcm))

	cLang := C.CString(lang)
	defer C.free(unsafe.Pointer(cLang))

	out := C.ww_full(m.ctx, ptr, clen, cLang)
	if out == nil {
		return "", fmt.Errorf("whisper inference failed")
	}
	defer C.ww_free_string(out)
	return C.GoString(out), nil
}

// Close releases the WhisperModel
func (m *WhisperModel) Close() {
	C.ww_free(m.ctx)
}
