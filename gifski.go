package gifski

// #cgo linux CFLAGS: -I.
// #cgo linux LDFLAGS: -L. -lgifski -lm -ldl
// #cgo darwin CFLAGS: -I.
// #cgo darwin LDFLAGS: -L. -lgifski_darwin -lm -ldl
// #include <stdlib.h>
// #include "gifski.h"
// typedef int (*write_callback_fn)(size_t buffer_length, const uint8_t *buffer, void *user_data);
// int writeCallback(int buffer_length, void *buffer, void *user_data);
// static int _gifski_set_write_callback(gifski *handle, void* user_data)
// {
//	 return gifski_set_write_callback(handle, (write_callback_fn)writeCallback, user_data);
// }
// typedef int (*progress_callback_fn)(void *user_data);
// int progressCallback(void *user_data);
// static void _gifski_set_progress_callback(gifski *handle, void* user_data)
// {
//	 gifski_set_progress_callback(handle, (progress_callback_fn)progressCallback, user_data);
// }
import "C"
import (
	"errors"
	"fmt"
	"io"
	"unsafe"

	"github.com/mattn/go-pointer"
)

// Settings controls the operation of gifski
type Settings struct {
	// Resize to max this width if non-0
	Width uint
	// Resize to max this height if width is non-0. Note that aspect ratio is not preserved.
	Height uint
	// 1-100, but useful range is 50-100. Recommended to set to 100.
	Quality uint
	// If true, looping is disabled. Recommended false (looping on).
	Once bool
	// Lower quality, but faster encode.
	Fast bool
	// should we report progress (if so, then the progress channel must be consumed)
	ReportProgress bool
}

// ErrorCode is a gifski internal error code
type ErrorCode int

const (
	// Ok is the success error code
	Ok ErrorCode = iota
	// NullArg one of input arguments was NULL
	NullArg
	// InvalidState a one-time function was called twice, or functions were called in wrong order
	InvalidState
	// Quant internal error related to palette quantization
	Quant
	// GIF internal error related to gif composing
	GIF
	// ThreadLost internal error related to multithreading
	ThreadLost
	// NotFound I/O error: file or directory not found
	NotFound
	// PermissionDenied I/O error: permission denied
	PermissionDenied
	// AlreadyExists I/O error: file already exists
	AlreadyExists
	// InvalidInput invalid arguments passed to function
	InvalidInput
	// TimedOut misc I/O error
	TimedOut
	// WriteZero misc I/O error
	WriteZero
	// Interrupted misc I/O error
	Interrupted
	// UnexpectedEOF misc I/O error
	UnexpectedEOF
	// Aborted progress callback returned 0, writing aborted
	Aborted
	// Other should not happen, file a bug
	Other
)

// Error is a gifski specific error
type Error struct {
	Code ErrorCode
}

func (e *Error) Error() string {
	return fmt.Sprintf("Internal gifski error code '%d'", e.Code)
}

// Gifski abstracts the gifski API
type Gifski struct {
	handle      *C.gifski
	ptr         unsafe.Pointer
	writer      io.Writer
	progress    chan uint
	frameNumber uint
}

//export writeCallback
func writeCallback(bufferLength C.int, buffer unsafe.Pointer, userData unsafe.Pointer) C.int {
	if bufferLength != 0 {
		g := pointer.Restore(userData).(*Gifski)
		data := C.GoBytes(buffer, bufferLength)
		bytesWritten := 0
		for bytesWritten != len(data) {
			n, err := g.writer.Write(data[bytesWritten:len(data)])
			if err != nil {
				return C.int(Other)
			}
			bytesWritten = bytesWritten + n
		}
	}
	return C.int(Ok)
}

//export progressCallback
func progressCallback(userData unsafe.Pointer) C.int {
	g := pointer.Restore(userData).(*Gifski)
	g.progress <- g.frameNumber
	g.frameNumber = g.frameNumber + 1
	// returning '0' means 'abort'
	return C.int(1)
}

// NewGifski creates a new instance of gifski with the specified settings
func NewGifski(settings *Settings, writer io.Writer) (*Gifski, error) {
	s := C.GifskiSettings{
		width:   C.uint(settings.Width),
		height:  C.uint(settings.Height),
		quality: C.uchar(settings.Quality),
		once:    C.bool(settings.Once),
		fast:    C.bool(settings.Fast),
	}
	handle := C.gifski_new(&s)
	if handle == nil {
		return nil, errors.New("failed to create new gifski instance")
	}
	g := &Gifski{
		handle:      handle,
		writer:      writer,
		progress:    make(chan uint),
		frameNumber: 0,
	}
	// Let's talk about this next bit; basically we can't pass a go function as a pointer through CGO, so instead
	//  we will call the helper C._gifski_set_write_callback() declared at the top of this file, that is like the
	//  gifski function gifski_set_write_callback() except it does not take the callback function as an argument.
	//  The C._gifski_set_write_callback() helper does have access to the Go callback function writeCallback() as
	//  it is statically passed through CGO with the "//export" syntax above, so it can call the gifski function
	//  gifski_set_write_callback() and pass it the statically passed Go callback function writeCallback(). We are
	//  saving the "pointer" so that we can later unref it so that it will be garbage collected.
	g.ptr = pointer.Save(g)
	errorCode := C._gifski_set_write_callback(handle, g.ptr)
	if ErrorCode(errorCode) != Ok {
		defer pointer.Unref(g.ptr)
		errorCode := C.gifski_finish(g.handle)
		close(g.progress)
		return nil, &Error{Code: ErrorCode(errorCode)}
	}
	if settings.ReportProgress {
		C._gifski_set_progress_callback(handle, g.ptr)
	}
	return g, nil
}

// AddFrame adds a frame to the animation. This function is asynchronous.
func (g *Gifski) AddFrame(frameNumber uint, width uint, height uint, pixels []byte, presentationTimeStamp float64) error {
	errorCode := C.gifski_add_frame_rgba(g.handle, C.uint(frameNumber), C.uint(width), C.uint(height), (*C.uint8_t)(unsafe.Pointer(&pixels[0])), C.double(presentationTimeStamp))
	if ErrorCode(errorCode) == Ok {
		return nil
	}
	return &Error{Code: ErrorCode(errorCode)}
}

// Progress gets the progress channel which reports the frame index
func (g *Gifski) Progress() chan uint {
	return g.progress
}

// Finish returns final status of write operations.
func (g *Gifski) Finish() error {
	defer pointer.Unref(g.ptr)
	errorCode := C.gifski_finish(g.handle)
	if g.progress != nil {
		close(g.progress)
	}
	if ErrorCode(errorCode) == Ok {
		return nil
	}
	return &Error{Code: ErrorCode(errorCode)}
}
