package main

/*
#include <stdlib.h>
extern void C_StreamCallback(const char* eventType, const char* data);
*/
import "C"
import (
	"unsafe"
)

// pushStreamEvent sends an SSE event to C++ via the C callback.
func pushStreamEvent(eventType string, data string) {
	cType := C.CString(eventType)
	cData := C.CString(data)
	defer C.free(unsafe.Pointer(cType))
	defer C.free(unsafe.Pointer(cData))

	C.C_StreamCallback(cType, cData)
}

// PushStreamChunk is a helper to push a text chunk.
func PushStreamChunk(chunk string) {
	pushStreamEvent("chunk", chunk)
}

// PushStreamDone signals the end of the stream.
func PushStreamDone() {
	pushStreamEvent("done", "")
}

// PushStreamError signals an error.
func PushStreamError(err string) {
	pushStreamEvent("error", err)
}
