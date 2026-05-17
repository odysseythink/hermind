package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"unsafe"
)

//export HermindInit
func HermindInit(configPathC *C.char) *C.char {
	_ = C.GoString(configPathC)
	status := map[string]string{"status": "ok", "version": "0.0.1-cgo"}
	data, _ := json.Marshal(status)
	p := C.malloc(C.size_t(len(data)))
	copy(unsafe.Slice((*byte)(p), len(data)), data)
	return (*C.char)(p)
}

//export HermindCall
func HermindCall(methodC *C.char, pathC *C.char, bodyC *C.char, bodyLen C.int, respLen *C.int) unsafe.Pointer {
	method := C.GoString(methodC)
	path := C.GoString(pathC)

	var bodyData []byte
	if bodyC != nil && bodyLen > 0 {
		bodyData = C.GoBytes(unsafe.Pointer(bodyC), bodyLen)
	}

	resp := handleRequest(method, path, bodyData)
	data, _ := json.Marshal(resp)
	p := C.malloc(C.size_t(len(data)))
	copy(unsafe.Slice((*byte)(p), len(data)), data)
	*respLen = C.int(len(data))
	return p
}

//export HermindFree
func HermindFree(p unsafe.Pointer) {
	C.free(p)
}

//export HermindSetStreamCallback
func HermindSetStreamCallback(callback unsafe.Pointer) {
	_ = callback
}

func main() {}

type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Code  int         `json:"code,omitempty"`
}

func handleRequest(method, path string, body []byte) response {
	if method == "GET" && path == "/health" {
		return response{OK: true, Data: map[string]string{"status": "healthy"}}
	}
	if method == "GET" && path == "/api/status" {
		return response{OK: true, Data: map[string]string{"status": "ok", "mode": "cgo"}}
	}
	return response{OK: false, Error: "not found", Code: 404}
}
