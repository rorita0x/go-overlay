package overlay

/*
#include "shim.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// Button is a mouse button, numbered as in linux/input-event-codes.h.
type Button uint32

const (
	ButtonLeft   Button = 0x110
	ButtonRight  Button = 0x111
	ButtonMiddle Button = 0x112
)

// Click is a mouse button released over a clickable label, at X, Y in the
// label's logical pixels.
type Click struct {
	Button Button
	X, Y   float64
}

// Pointer images to show over a surface (cursor-shape-v1 shapes).
const (
	cursorDefault = C.WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_DEFAULT
	cursorPointer = C.WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_POINTER // a hand: something to click
)

// pointerTarget is one of our surfaces that takes pointer input. Its
// methods run on Run's thread.
type pointerTarget interface {
	cursor() uint32 // the pointer image over it, see cursorDefault
	pointerEnter(x, y float64)
	pointerMotion(x, y float64)
	pointerButton(b Button, pressed bool)
	pointerLeave()
}

func surfaceKey(wl *C.struct_wl_surface) uintptr {
	return uintptr(unsafe.Pointer(wl))
}

// addPointerTarget routes the pointer events of wl to t.
func (o *Overlay) addPointerTarget(wl *C.struct_wl_surface, t pointerTarget) {
	if o.pointerTargets == nil {
		o.pointerTargets = map[uintptr]pointerTarget{}
	}
	o.pointerTargets[surfaceKey(wl)] = t
}

// removePointerTarget stops routing to wl's target; call before destroying
// wl.
func (o *Overlay) removePointerTarget(wl *C.struct_wl_surface) {
	key := surfaceKey(wl)
	if t, ok := o.pointerTargets[key]; ok && o.pointerFocus == t {
		o.pointerFocus = nil
	}
	delete(o.pointerTargets, key)
}

func (o *Overlay) pointerEnter(surf uintptr, x, y float64) {
	o.pointerFocus = o.pointerTargets[surf]
	if o.pointerFocus != nil {
		o.pointerFocus.pointerEnter(x, y)
	}
}

// pointerLeave ends the pointer focus; surf 0 means whatever has it.
func (o *Overlay) pointerLeave(surf uintptr) {
	t := o.pointerFocus
	if t == nil || surf != 0 && o.pointerTargets[surf] != t {
		return
	}
	o.pointerFocus = nil
	t.pointerLeave()
}

func (o *Overlay) pointerMotion(x, y float64) {
	if t := o.pointerFocus; t != nil {
		t.pointerMotion(x, y)
	}
}

func (o *Overlay) pointerButton(button, state uint32) {
	if t := o.pointerFocus; t != nil {
		t.pointerButton(Button(button), state == C.WL_POINTER_BUTTON_STATE_PRESSED)
	}
}

//export ovPointerEnter
func ovPointerEnter(h C.uintptr_t, serial C.uint32_t, surf C.uintptr_t, x, y C.double) {
	s := cgo.Handle(h).Value().(*seat)
	s.o.pointerEnter(uintptr(surf), float64(x), float64(y))
	if t := s.o.pointerFocus; t != nil {
		s.setCursor(uint32(serial), t.cursor())
	}
}

//export ovPointerLeave
func ovPointerLeave(h C.uintptr_t, surf C.uintptr_t) {
	cgo.Handle(h).Value().(*seat).o.pointerLeave(uintptr(surf))
}

//export ovPointerMotion
func ovPointerMotion(h C.uintptr_t, x, y C.double) {
	cgo.Handle(h).Value().(*seat).o.pointerMotion(float64(x), float64(y))
}

//export ovPointerButton
func ovPointerButton(h C.uintptr_t, button, state C.uint32_t) {
	cgo.Handle(h).Value().(*seat).o.pointerButton(uint32(button), uint32(state))
}
