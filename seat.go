package overlay

/*
#cgo pkg-config: xkbcommon
#include <xkbcommon/xkbcommon.h>
#include "shim.h"
*/
import "C"

import (
	"runtime/cgo"
	"syscall"
	"unsafe"
)

// Key is a key-press event delivered to OnKey.
type Key struct {
	Sym  uint32
	Rune rune
}

const (
	KeyLeft   = 0xff51
	KeyUp     = 0xff52
	KeyRight  = 0xff53
	KeyDown   = 0xff54
	KeyReturn = 0xff0d
	KeyEscape = 0xff1b
)

// FKey returns the keysym for function key n (1-35), e.g. FKey(1) == F1.
func FKey(n int) uint32 {
	return 0xffbe + uint32(n-1)
}

type seat struct {
	o        *Overlay
	wl       *C.struct_wl_seat
	handle   cgo.Handle
	keyboard *C.struct_wl_keyboard
	kbHandle cgo.Handle
	pointer  *C.struct_wl_pointer
	ptHandle cgo.Handle
	shape    *C.struct_wp_cursor_shape_device_v1 // made on first use

	xkbCtx    *C.struct_xkb_context
	xkbKeymap *C.struct_xkb_keymap
	xkbState  *C.struct_xkb_state
}

func (s *seat) capabilities(caps uint32) {
	has := caps&C.WL_SEAT_CAPABILITY_KEYBOARD != 0
	switch {
	case has && s.keyboard == nil:
		s.keyboard = C.wl_seat_get_keyboard(s.wl)
		s.kbHandle = cgo.NewHandle(s)
		C.ov_keyboard_add_listener(s.keyboard, C.uintptr_t(s.kbHandle))
	case !has && s.keyboard != nil:
		C.wl_keyboard_release(s.keyboard)
		s.kbHandle.Delete()
		s.keyboard = nil
	}

	has = caps&C.WL_SEAT_CAPABILITY_POINTER != 0
	switch {
	case has && s.pointer == nil:
		s.pointer = C.wl_seat_get_pointer(s.wl)
		s.ptHandle = cgo.NewHandle(s)
		C.ov_pointer_add_listener(s.pointer, C.uintptr_t(s.ptHandle))
	case !has && s.pointer != nil:
		if s.shape != nil {
			C.wp_cursor_shape_device_v1_destroy(s.shape)
			s.shape = nil
		}
		C.wl_pointer_release(s.pointer)
		s.ptHandle.Delete()
		s.pointer = nil
		s.o.pointerLeave(0)
	}
}

// setCursor shows the compositor's own pointer image of the given
// cursor-shape-v1 shape; serial is that of the pointer's enter event. Without
// it the compositor may hide the pointer over our surfaces. A no-op if the
// compositor lacks cursor-shape-v1 or shape is 0.
func (s *seat) setCursor(serial, shape uint32) {
	if shape == 0 || s.pointer == nil || s.o.cursorShape == nil {
		return
	}
	if s.shape == nil {
		s.shape = C.wp_cursor_shape_manager_v1_get_pointer(s.o.cursorShape, s.pointer)
	}
	C.wp_cursor_shape_device_v1_set_shape(s.shape, C.uint32_t(serial), C.uint32_t(shape))
}

func (s *seat) setKeymap(data []byte) {
	if s.xkbState != nil {
		C.xkb_state_unref(s.xkbState)
		s.xkbState = nil
	}
	if s.xkbKeymap != nil {
		C.xkb_keymap_unref(s.xkbKeymap)
		s.xkbKeymap = nil
	}
	if s.xkbCtx == nil {
		s.xkbCtx = C.xkb_context_new(C.XKB_CONTEXT_NO_FLAGS)
	}
	if len(data) == 0 {
		return
	}
	s.xkbKeymap = C.xkb_keymap_new_from_string(s.xkbCtx,
		(*C.char)(unsafe.Pointer(&data[0])), C.XKB_KEYMAP_FORMAT_TEXT_V1, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if s.xkbKeymap == nil {
		return
	}
	s.xkbState = C.xkb_state_new(s.xkbKeymap)
}

func (s *seat) modifiers(depressed, latched, locked, group uint32) {
	if s.xkbState == nil {
		return
	}
	C.xkb_state_update_mask(s.xkbState,
		C.xkb_mod_mask_t(depressed), C.xkb_mod_mask_t(latched), C.xkb_mod_mask_t(locked),
		0, 0, C.xkb_layout_index_t(group))
}

func (s *seat) key(code, state uint32) {
	if state != C.WL_KEYBOARD_KEY_STATE_PRESSED || s.xkbState == nil {
		return
	}
	// +8: evdev-to-XKB keycode offset.
	sym := C.xkb_state_key_get_one_sym(s.xkbState, C.xkb_keycode_t(code+8))
	if s.o.onKey != nil {
		s.o.onKey(Key{Sym: uint32(sym), Rune: rune(C.xkb_keysym_to_utf32(sym))})
	}
}

//export ovSeatCapabilities
func ovSeatCapabilities(h C.uintptr_t, caps C.uint32_t) {
	cgo.Handle(h).Value().(*seat).capabilities(uint32(caps))
}

//export ovKeyboardKeymap
func ovKeyboardKeymap(h C.uintptr_t, format C.uint32_t, fd C.int32_t, size C.uint32_t) {
	defer syscall.Close(int(fd))
	if format != C.WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 {
		return
	}
	data, err := syscall.Mmap(int(fd), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return
	}
	defer syscall.Munmap(data)
	cgo.Handle(h).Value().(*seat).setKeymap(data)
}

//export ovKeyboardKey
func ovKeyboardKey(h C.uintptr_t, key, state C.uint32_t) {
	cgo.Handle(h).Value().(*seat).key(uint32(key), uint32(state))
}

//export ovKeyboardModifiers
func ovKeyboardModifiers(h C.uintptr_t, depressed, latched, locked, group C.uint32_t) {
	cgo.Handle(h).Value().(*seat).modifiers(uint32(depressed), uint32(latched), uint32(locked), uint32(group))
}
