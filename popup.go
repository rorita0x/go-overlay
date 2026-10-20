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

// Popup is a single, centered, keyboard-focused overlay surface.
type Popup struct {
	Width, Height int
}

type popupRequest struct {
	size  Popup
	label *Label
}

// SetPopup shows or updates the popup. Safe to call from any goroutine.
func (o *Overlay) SetPopup(p Popup, l Label) {
	cl := l.clone()
	o.mu.Lock()
	o.popupReq = &popupRequest{size: p, label: &cl}
	o.popupGen++
	o.mu.Unlock()
	o.wake()
}

// ClearPopup hides the popup. Safe to call from any goroutine.
func (o *Overlay) ClearPopup() {
	o.mu.Lock()
	o.popupReq = nil
	o.popupGen++
	o.mu.Unlock()
	o.wake()
}

func (o *Overlay) syncPopup() {
	o.mu.Lock()
	req, gen := o.popupReq, o.popupGen
	o.mu.Unlock()

	switch {
	case req == nil:
		if o.popup != nil {
			o.popup.destroy()
			o.popup = nil
		}
	case o.popup == nil:
		o.popup = newPopup(o, req, gen)
	case o.popup.gen != gen:
		o.popup.setContent(req, gen)
	}
}

type popup struct {
	o      *Overlay
	wl     *C.struct_wl_surface
	layer  *C.struct_zwlr_layer_surface_v1
	handle cgo.Handle

	req  *popupRequest
	gen  uint64
	w, h int

	preferredScale int
	configured     bool
	resizing       bool
	dirty          bool
	buffers        []*popupBuffer
}

func newPopup(o *Overlay, req *popupRequest, gen uint64) *popup {
	p := &popup{o: o, req: req, gen: gen, w: req.size.Width, h: req.size.Height}
	p.handle = cgo.NewHandle(p)
	p.wl = C.wl_compositor_create_surface(o.compositor)
	C.ov_surface_add_listener(p.wl, C.uintptr_t(p.handle))

	p.layer = C.zwlr_layer_shell_v1_get_layer_surface(o.layerShell, p.wl, nil,
		C.ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, o.namespace)
	C.ov_layer_surface_add_listener(p.layer, C.uintptr_t(p.handle))

	C.zwlr_layer_surface_v1_set_anchor(p.layer, 0)
	C.zwlr_layer_surface_v1_set_size(p.layer, C.uint32_t(p.w), C.uint32_t(p.h))
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(p.layer,
		C.ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_EXCLUSIVE)
	C.wl_surface_commit(p.wl)
	return p
}

func (p *popup) setContent(req *popupRequest, gen uint64) {
	old := p.req
	p.req, p.gen = req, gen
	if req.size.Width != old.size.Width || req.size.Height != old.size.Height {
		p.w, p.h = req.size.Width, req.size.Height
		C.zwlr_layer_surface_v1_set_size(p.layer, C.uint32_t(p.w), C.uint32_t(p.h))
		C.wl_surface_commit(p.wl)
		p.dirty, p.resizing = true, true
		return
	}
	p.draw()
}

func (p *popup) scale() int { return max(p.preferredScale, 1) }

func (p *popup) setPreferredScale(f int) {
	if f != p.preferredScale {
		p.preferredScale = f
		p.draw()
	}
}

func (p *popup) configure(serial uint32) {
	C.zwlr_layer_surface_v1_ack_configure(p.layer, C.uint32_t(serial))
	p.configured, p.resizing = true, false
	p.draw()
}

func (p *popup) closed() {
	p.destroy()
	p.o.popup = nil
}

func (p *popup) draw() {
	p.dirty = true
	if !p.configured || p.resizing {
		return
	}
	sc := p.scale()
	w, h := p.w*sc, p.h*sc
	b := p.freeBuffer(w, h)
	if b == nil {
		return
	}
	render(p.req.label, TopLeft, b.data, w, h, b.stride, sc)
	C.wl_surface_set_buffer_scale(p.wl, C.int32_t(sc))
	C.wl_surface_attach(p.wl, b.wl, 0, 0)
	C.wl_surface_damage_buffer(p.wl, 0, 0, C.int32_t(w), C.int32_t(h))
	C.wl_surface_commit(p.wl)
	b.busy = true
	p.dirty = false
}

func (p *popup) freeBuffer(w, h int) *popupBuffer {
	keep := p.buffers[:0]
	var found *popupBuffer
	for _, b := range p.buffers {
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
	p.buffers = keep
	if found != nil || len(p.buffers) >= 2 {
		return found
	}
	b, err := newPopupBuffer(p, w, h)
	if err != nil {
		p.o.fail(err)
		return nil
	}
	p.buffers = append(p.buffers, b)
	return b
}

func (p *popup) destroy() {
	for _, b := range p.buffers {
		b.destroy()
	}
	p.buffers = nil
	C.zwlr_layer_surface_v1_destroy(p.layer)
	C.wl_surface_destroy(p.wl)
	p.handle.Delete()
}

type popupBuffer struct {
	p         *popup
	wl        *C.struct_wl_buffer
	handle    cgo.Handle
	data      unsafe.Pointer
	size      int
	w, h      int
	stride    int
	busy      bool
	destroyed bool
}

func newPopupBuffer(p *popup, w, h int) (*popupBuffer, error) {
	b := &popupBuffer{p: p, w: w, h: h, stride: w * 4}
	b.size = b.stride * h
	fd, err := C.ov_shm_alloc(C.size_t(b.size), &b.data)
	if fd < 0 {
		return nil, fmt.Errorf("overlay: allocate %dx%d popup shm buffer: %w", w, h, err)
	}
	pool := C.wl_shm_create_pool(p.o.shm, fd, C.int32_t(b.size))
	b.wl = C.wl_shm_pool_create_buffer(pool, 0, C.int32_t(w), C.int32_t(h), C.int32_t(b.stride), C.WL_SHM_FORMAT_ARGB8888)
	C.wl_shm_pool_destroy(pool)
	syscall.Close(int(fd))
	b.handle = cgo.NewHandle(b)
	C.ov_buffer_add_listener(b.wl, C.uintptr_t(b.handle))
	return b, nil
}

func (b *popupBuffer) release() {
	b.busy = false
	if b.p.dirty {
		b.p.draw()
	}
}

func (b *popupBuffer) destroy() {
	if b.destroyed {
		return
	}
	b.destroyed = true
	C.wl_buffer_destroy(b.wl)
	C.ov_shm_free(b.data, C.size_t(b.size))
	b.handle.Delete()
}
