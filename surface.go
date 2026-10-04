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

// surface is one corner label on one output.
type surface struct {
	out    *output
	corner Corner
	wl     *C.struct_wl_surface
	layer  *C.struct_zwlr_layer_surface_v1
	handle cgo.Handle

	label *Label
	gen   uint64
	w, h  int // logical size passed to set_size

	preferredScale int // from wl_surface.preferred_buffer_scale, 0 = not sent
	configured     bool
	resizing       bool // set_size sent, waiting for its configure
	dirty          bool // needs a redraw once a buffer is free
	buffers        []*buffer
}

func newSurface(out *output, c Corner, l *Label, gen uint64) *surface {
	o := out.o
	s := &surface{out: out, corner: c, label: l, gen: gen}
	s.handle = cgo.NewHandle(s)
	s.wl = C.wl_compositor_create_surface(o.compositor)
	C.ov_surface_add_listener(s.wl, C.uintptr_t(s.handle))

	// An empty input region lets pointer events fall through to what's below.
	region := C.wl_compositor_create_region(o.compositor)
	C.wl_surface_set_input_region(s.wl, region)
	C.wl_region_destroy(region)

	s.layer = C.zwlr_layer_shell_v1_get_layer_surface(o.layerShell, s.wl, out.wl,
		C.ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, o.namespace)
	C.ov_layer_surface_add_listener(s.layer, C.uintptr_t(s.handle))

	anchor := C.uint32_t(C.ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM)
	if c.top() {
		anchor = C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP
	}
	if c.left() {
		anchor |= C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT
	} else {
		anchor |= C.ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT
	}
	C.zwlr_layer_surface_v1_set_anchor(s.layer, anchor)
	m := C.int32_t(l.Style.Margin)
	C.zwlr_layer_surface_v1_set_margin(s.layer, m, m, m, m)
	C.zwlr_layer_surface_v1_set_exclusive_zone(s.layer, C.int32_t(o.opts.ExclusiveZone))
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(s.layer, C.ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE)
	s.w, s.h = measure(l)
	C.zwlr_layer_surface_v1_set_size(s.layer, C.uint32_t(s.w), C.uint32_t(s.h))
	// The initial commit without a buffer asks for the first configure.
	C.wl_surface_commit(s.wl)
	return s
}

func (s *surface) setLabel(l *Label, gen uint64) {
	old := s.label
	s.label, s.gen = l, gen
	if l.Style.Margin != old.Style.Margin {
		m := C.int32_t(l.Style.Margin)
		C.zwlr_layer_surface_v1_set_margin(s.layer, m, m, m, m)
	}
	if w, h := measure(l); w != s.w || h != s.h {
		// Draw after the compositor confirms the new size.
		s.w, s.h = w, h
		C.zwlr_layer_surface_v1_set_size(s.layer, C.uint32_t(w), C.uint32_t(h))
		C.wl_surface_commit(s.wl)
		s.dirty, s.resizing = true, true
		return
	}
	s.draw()
}

func (s *surface) scale() int {
	switch {
	case s.preferredScale > 0:
		return s.preferredScale
	case s.out.scale > 0:
		return s.out.scale
	}
	return 1
}

func (s *surface) setPreferredScale(f int) {
	if f != s.preferredScale {
		s.preferredScale = f
		s.draw()
	}
}

func (s *surface) configure(serial uint32) {
	C.zwlr_layer_surface_v1_ack_configure(s.layer, C.uint32_t(serial))
	s.configured, s.resizing = true, false
	s.draw()
}

// closed is sent when the compositor drops the surface, e.g. because its
// output went away. The corner stays hidden until its label changes again.
func (s *surface) closed() {
	out := s.out
	out.closedGen[s.corner] = s.gen
	s.destroy()
	out.surfaces[s.corner] = nil
}

func (s *surface) draw() {
	s.dirty = true
	if !s.configured || s.resizing {
		return
	}
	sc := s.scale()
	w, h := s.w*sc, s.h*sc
	b := s.freeBuffer(w, h)
	if b == nil {
		return // redrawn on the next buffer release
	}
	render(s.label, s.corner, b.data, w, h, b.stride, sc)
	C.wl_surface_set_buffer_scale(s.wl, C.int32_t(sc))
	C.wl_surface_attach(s.wl, b.wl, 0, 0)
	C.wl_surface_damage_buffer(s.wl, 0, 0, C.int32_t(w), C.int32_t(h))
	C.wl_surface_commit(s.wl)
	b.busy = true
	s.dirty = false
}

// freeBuffer returns an idle buffer of the given pixel size, creating one if
// fewer than two exist. It returns nil if both are held by the compositor.
func (s *surface) freeBuffer(w, h int) *buffer {
	keep := s.buffers[:0]
	var found *buffer
	for _, b := range s.buffers {
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
	s.buffers = keep
	if found != nil || len(s.buffers) >= 2 {
		return found
	}
	b, err := newBuffer(s, w, h)
	if err != nil {
		s.out.o.fail(err)
		return nil
	}
	s.buffers = append(s.buffers, b)
	return b
}

func (s *surface) destroy() {
	for _, b := range s.buffers {
		// The memory is never reused, so destroying a buffer that's still
		// attached is fine.
		b.destroy()
	}
	s.buffers = nil
	C.zwlr_layer_surface_v1_destroy(s.layer)
	C.wl_surface_destroy(s.wl)
	s.handle.Delete()
}

// buffer is a wl_shm buffer backed by its own memfd.
type buffer struct {
	s         *surface
	wl        *C.struct_wl_buffer
	handle    cgo.Handle
	data      unsafe.Pointer
	size      int
	w, h      int
	stride    int
	busy      bool
	destroyed bool
}

func newBuffer(s *surface, w, h int) (*buffer, error) {
	b := &buffer{s: s, w: w, h: h, stride: w * 4}
	b.size = b.stride * h
	fd, err := C.ov_shm_alloc(C.size_t(b.size), &b.data)
	if fd < 0 {
		return nil, fmt.Errorf("overlay: allocate %dx%d shm buffer: %w", w, h, err)
	}
	pool := C.wl_shm_create_pool(s.out.o.shm, fd, C.int32_t(b.size))
	b.wl = C.wl_shm_pool_create_buffer(pool, 0, C.int32_t(w), C.int32_t(h), C.int32_t(b.stride), C.WL_SHM_FORMAT_ARGB8888)
	C.wl_shm_pool_destroy(pool)
	syscall.Close(int(fd))
	b.handle = cgo.NewHandle(b)
	C.ov_buffer_add_listener(b.wl, C.uintptr_t(b.handle))
	return b, nil
}

func (b *buffer) release() {
	b.busy = false
	if b.s.dirty {
		b.s.draw()
	}
}

func (b *buffer) destroy() {
	if b.destroyed {
		return
	}
	b.destroyed = true
	C.wl_buffer_destroy(b.wl)
	C.ov_shm_free(b.data, C.size_t(b.size))
	b.handle.Delete()
}
