package overlay

/*
#include "shim.h"
*/
import "C"

import (
	"runtime/cgo"
	"time"
)

// radialRun is the open radial menu: its state and one transparent,
// full-output surface per output in use. Owned by Run's thread.
type radialRun struct {
	gen      uint64
	menu     *radialMenu
	surfaces []*radialSurface
	stopTick chan struct{} // closed to stop the Refresh ticker; nil without one
}

// syncRadial opens, replaces or closes the radial menu to match the last
// SetRadial/ClearRadial, and redraws it on a Refresh tick.
func (o *Overlay) syncRadial() {
	o.mu.Lock()
	req, gen := o.radialReq, o.radialGen
	o.mu.Unlock()

	if run := o.radial; run != nil && run.gen == gen {
		if o.radialTick.Swap(false) {
			run.redraw()
		}
		return
	}
	o.destroyRadial()
	if req == nil {
		return
	}

	run := &radialRun{gen: gen, menu: newRadialMenu(*req)}
	run.menu.redraw = run.redraw
	run.menu.close = func() { o.endRadial(gen) }
	for _, out := range o.outputs {
		if out.ready && o.wants(out) {
			run.surfaces = append(run.surfaces, newRadialSurface(o, out))
		}
	}
	if d := req.Refresh; d > 0 {
		run.stopTick = make(chan struct{})
		go func(stop <-chan struct{}) {
			t := time.NewTicker(d)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					o.radialTick.Store(true)
					o.wake()
				case <-stop:
					return
				}
			}
		}(run.stopTick)
	}
	o.radial = run
}

// endRadial closes the menu from inside, after a pick or a cancel: its
// request is dropped too, unless a newer SetRadial replaced it already.
func (o *Overlay) endRadial(gen uint64) {
	o.destroyRadial()
	o.mu.Lock()
	if o.radialGen == gen {
		o.radialReq = nil
		o.radialShown.Store(false)
	}
	o.mu.Unlock()
}

// destroyRadial removes the open menu's surfaces, without callbacks.
func (o *Overlay) destroyRadial() {
	run := o.radial
	if run == nil {
		return
	}
	o.radial = nil
	if run.stopTick != nil {
		close(run.stopTick)
	}
	for _, s := range run.surfaces {
		s.destroy()
	}
	o.radialTick.Store(false)
}

func (run *radialRun) redraw() {
	if s := run.menu.owner; s != nil {
		s.draw()
	}
}

// radialSurface covers one output while a radial menu is open: transparent
// except for the ring (on the output the pointer was on), taking every
// pointer event so a click beside the ring cancels the menu, and never the
// keyboard focus.
type radialSurface struct {
	o      *Overlay
	out    *output
	wl     *C.struct_wl_surface
	layer  *C.struct_zwlr_layer_surface_v1
	handle cgo.Handle

	w, h           int // logical size, from configure
	preferredScale int
	configured     bool
	dirty          bool
	fullDamage     bool // the next frame damages the whole buffer
	shown          box  // the ring's area in the last frame, buffer pixels
	buffers        []*shmBuffer
}

func newRadialSurface(o *Overlay, out *output) *radialSurface {
	s := &radialSurface{o: o, out: out, fullDamage: true}
	s.handle = cgo.NewHandle(s)
	s.wl = C.wl_compositor_create_surface(o.compositor)
	C.ov_surface_add_listener(s.wl, C.uintptr_t(s.handle))
	s.layer = C.zwlr_layer_shell_v1_get_layer_surface(o.layerShell, s.wl, out.wl,
		C.ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, o.namespace)
	C.ov_layer_surface_add_listener(s.layer, C.uintptr_t(s.handle))
	C.zwlr_layer_surface_v1_set_anchor(s.layer, C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|
		C.ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT|
		C.ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT)
	C.zwlr_layer_surface_v1_set_size(s.layer, 0, 0) // the whole output
	C.zwlr_layer_surface_v1_set_exclusive_zone(s.layer, -1)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(s.layer, C.ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE)
	// The default input region (all of it) is kept: clicks beside the ring
	// must reach the menu, not the window below.
	C.wl_surface_commit(s.wl)
	o.addPointerTarget(s.wl, s)
	return s
}

// menu returns the open menu if s belongs to it.
func (s *radialSurface) menu() *radialMenu {
	run := s.o.radial
	if run == nil {
		return nil
	}
	for _, rs := range run.surfaces {
		if rs == s {
			return run.menu
		}
	}
	return nil
}

func (s *radialSurface) cursor() uint32 { return cursorDefault }

func (s *radialSurface) pointerEnter(x, y float64) {
	if m := s.menu(); m != nil {
		m.enter(s, x, y, float64(s.w), float64(s.h))
	}
}

func (s *radialSurface) pointerMotion(x, y float64) {
	if m := s.menu(); m != nil {
		m.motion(s, x, y)
	}
}

func (s *radialSurface) pointerButton(b Button, pressed bool) {
	if m := s.menu(); m != nil {
		m.button(s, b, pressed)
	}
}

func (s *radialSurface) pointerLeave() {
	if m := s.menu(); m != nil {
		m.leave(s)
	}
}

func (s *radialSurface) configure(serial, w, h uint32) {
	C.zwlr_layer_surface_v1_ack_configure(s.layer, C.uint32_t(serial))
	if int(w) != s.w || int(h) != s.h {
		s.w, s.h = int(w), int(h)
		s.fullDamage = true
	}
	s.configured = true
	s.draw()
}

// closed is sent when the compositor drops the surface, e.g. because its
// output went away. If the ring was on it, the menu is cancelled.
func (s *radialSurface) closed() {
	run := s.o.radial
	if run == nil {
		return
	}
	if m := run.menu; m.owner == s && !m.done {
		m.act(target{zone: zoneOutside}, 0)
		return
	}
	for i, rs := range run.surfaces {
		if rs == s {
			run.surfaces = append(run.surfaces[:i], run.surfaces[i+1:]...)
			break
		}
	}
	s.destroy()
}

func (s *radialSurface) scale() int {
	switch {
	case s.preferredScale > 0:
		return s.preferredScale
	case s.out.scale > 0:
		return s.out.scale
	}
	return 1
}

func (s *radialSurface) setPreferredScale(f int) {
	if f != s.preferredScale {
		s.preferredScale = f
		s.fullDamage = true
		s.draw()
	}
}

func (s *radialSurface) bufferReleased() {
	if s.dirty {
		s.draw()
	}
}

func (s *radialSurface) draw() {
	s.dirty = true
	if !s.configured || s.w <= 0 || s.h <= 0 {
		return
	}
	sc := s.scale()
	w, h := s.w*sc, s.h*sc
	b := freeShmBuffer(s.o, s, &s.buffers, w, h)
	if b == nil {
		return // redrawn on the next buffer release
	}
	var view *radialView
	if m := s.menu(); m != nil && m.owner == s {
		if v, ok := m.view(); ok {
			view = &v
		}
	}
	// Every frame is drawn in full, so only what changed since the last
	// one needs damage: where the ring was and where it is now.
	drawn := renderRadial(b.data, w, h, b.stride, sc, view)
	damage := s.shown.union(drawn)
	if s.fullDamage {
		damage, s.fullDamage = box{0, 0, w, h}, false
	}
	s.shown = drawn
	C.wl_surface_set_buffer_scale(s.wl, C.int32_t(sc))
	C.wl_surface_attach(s.wl, b.wl, 0, 0)
	if !damage.empty() {
		C.wl_surface_damage_buffer(s.wl, C.int32_t(damage.x), C.int32_t(damage.y), C.int32_t(damage.w), C.int32_t(damage.h))
	}
	C.wl_surface_commit(s.wl)
	b.busy = true
	s.dirty = false
}

func (s *radialSurface) destroy() {
	for _, b := range s.buffers {
		b.destroy()
	}
	s.buffers = nil
	s.o.removePointerTarget(s.wl)
	C.zwlr_layer_surface_v1_destroy(s.layer)
	C.wl_surface_destroy(s.wl)
	s.handle.Delete()
}

// box is a rectangle in buffer pixels; the zero box is empty.
type box struct{ x, y, w, h int }

func (b box) empty() bool { return b.w <= 0 || b.h <= 0 }

func (b box) union(c box) box {
	switch {
	case b.empty():
		return c
	case c.empty():
		return b
	}
	x0, y0 := min(b.x, c.x), min(b.y, c.y)
	x1, y1 := max(b.x+b.w, c.x+c.w), max(b.y+b.h, c.y+c.h)
	return box{x0, y0, x1 - x0, y1 - y0}
}
