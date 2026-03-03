//go:build android

package main

/*
#cgo LDFLAGS: -lmediandk -llog
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
int writeMP4ToPath(int w, int h, int n, int *delays_cs, uint8_t **frameData, const char *path);
*/
import "C"
import (
	"fmt"
	"image"
	"image/draw"
	"unsafe"
)

func WriteMP4(frames []image.Image, delays []int, path string) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames to encode")
	}
	bounds := frames[0].Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	frameBytes := w * h * 4 // RGBA

	// Allocate delays array in C memory.
	cDelays := (*C.int)(C.malloc(C.size_t(len(delays)) * C.size_t(unsafe.Sizeof(C.int(0)))))
	defer C.free(unsafe.Pointer(cDelays))
	delaySlice := unsafe.Slice(cDelays, len(delays))
	for i, d := range delays {
		delaySlice[i] = C.int(d)
	}

	// Allocate a C array of frame-data pointers, and copy each frame's RGBA
	// pixels into C memory. This avoids passing Go pointers through C (which
	// violates CGo rules and panics at runtime with cgocheck=1).
	ptrSize := C.size_t(unsafe.Sizeof((*C.uint8_t)(nil)))
	cPtrArray := (**C.uint8_t)(C.malloc(C.size_t(len(frames)) * ptrSize))
	defer C.free(unsafe.Pointer(cPtrArray))
	ptrSlice := unsafe.Slice(cPtrArray, len(frames))

	// Track individual frame buffers so we can free them.
	frameBufs := make([]unsafe.Pointer, len(frames))
	defer func() {
		for _, p := range frameBufs {
			if p != nil {
				C.free(p)
			}
		}
	}()

	for i, img := range frames {
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		buf := C.malloc(C.size_t(frameBytes))
		C.memcpy(buf, unsafe.Pointer(&rgba.Pix[0]), C.size_t(frameBytes))
		frameBufs[i] = buf
		ptrSlice[i] = (*C.uint8_t)(buf)
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	ret := C.writeMP4ToPath(
		C.int(w), C.int(h),
		C.int(len(frames)),
		cDelays,
		cPtrArray,
		cPath,
	)
	if ret != 0 {
		return fmt.Errorf("MP4 encoding failed (code %d)", int(ret))
	}
	return nil
}
