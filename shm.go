package overlay

/*
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"syscall"
	"unsafe"
)

// bufferOwner is told when the compositor is done with one of its buffers,
// so a frame that waited for a free one can be drawn.
type bufferOwner interface{ bufferReleased() }

// shmBuffer is a wl_shm buffer backed by its own memfd, used by the popup
// and the radial menu.
type shmBuffer struct {
	owner     bufferOwner
	wl        *C.struct_wl_buffer
	handle    cgo.Handle
	data      unsafe.Pointer
	size      int
	w, h      int
	stride    int
	busy      bool
	destroyed bool
}

func newShmBuffer(o *Overlay, owner bufferOwner, w, h int) (*shmBuffer, error) {
	b := &shmBuffer{owner: owner, w: w, h: h, stride: w * 4}
	b.size = b.stride * h
	fd, err := C.ov_shm_alloc(C.size_t(b.size), &b.data)
	if fd < 0 {
		return nil, fmt.Errorf("overlay: allocate %dx%d shm buffer: %w", w, h, err)
	}
	pool := C.wl_shm_create_pool(o.shm, fd, C.int32_t(b.size))
	b.wl = C.wl_shm_pool_create_buffer(pool, 0, C.int32_t(w), C.int32_t(h), C.int32_t(b.stride), C.WL_SHM_FORMAT_ARGB8888)
	C.wl_shm_pool_destroy(pool)
	syscall.Close(int(fd))
	b.handle = cgo.NewHandle(b)
	C.ov_buffer_add_listener(b.wl, C.uintptr_t(b.handle))
	return b, nil
}

func (b *shmBuffer) release() {
	b.busy = false
	b.owner.bufferReleased()
}

func (b *shmBuffer) destroy() {
	if b.destroyed {
		return
	}
	b.destroyed = true
	C.wl_buffer_destroy(b.wl)
	C.ov_shm_free(b.data, C.size_t(b.size))
	b.handle.Delete()
}

// freeShmBuffer returns an idle buffer of w×h pixels from *bufs, creating one
// if fewer than two exist, and drops idle ones of another size. It returns
// nil if both are held by the compositor (or allocating failed, which it
// reports to o).
func freeShmBuffer(o *Overlay, owner bufferOwner, bufs *[]*shmBuffer, w, h int) *shmBuffer {
	keep := (*bufs)[:0]
	var found *shmBuffer
	for _, b := range *bufs {
		switch {
		case b.busy:
			keep = append(keep, b)
		case b.w != w || b.h != h:
			b.destroy()
		default:
			if found == nil {
				found = b
			}
			keep = append(keep, b)
		}
	}
	*bufs = keep
	if found != nil || len(*bufs) >= 2 {
		return found
	}
	b, err := newShmBuffer(o, owner, w, h)
	if err != nil {
		o.fail(err)
		return nil
	}
	*bufs = append(*bufs, b)
	return b
}
