// Package overlay draws text labels in the corners of the screen on Wayland
// compositors that support the wlr-layer-shell protocol (KWin, Sway,
// Hyprland, niri, …). Labels are click-through and can be updated at any time.
//
//	ov, err := overlay.New(overlay.Options{})
//	...
//	ov.Set(overlay.TopRight, overlay.Label{Spans: []overlay.Span{{Text: "hello"}}})
//	err = ov.Run(ctx)
package overlay

/*
#cgo pkg-config: wayland-client
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/cgo"
	"slices"
	"sync"
	"syscall"
	"unsafe"
)

// Options configure an Overlay.
type Options struct {
	// Namespace is the layer-surface namespace compositors use for rules.
	// Default "go-overlay".
	Namespace string
	// Outputs restricts the overlay to outputs with these names (e.g. "DP-1").
	// Empty means every output, including ones connected later.
	Outputs []string
	// ExclusiveZone 0 keeps labels clear of panels; -1 draws on top of them.
	ExclusiveZone int
}

// Overlay is a connection to the Wayland compositor that owns the corner labels.
type Overlay struct {
	opts      Options
	namespace *C.char

	display    *C.struct_wl_display
	registry   *C.struct_wl_registry
	compositor *C.struct_wl_compositor
	shm        *C.struct_wl_shm
	layerShell *C.struct_zwlr_layer_shell_v1
	handle     cgo.Handle
	outputs    map[uint32]*output
	loopErr    error // first error inside an event handler; ends Run

	wakeR, wakeW int

	mu     sync.Mutex
	labels [4]*Label // immutable once stored; nil = corner hidden
	gens   [4]uint64 // bumped on every Set/Clear
}

// New connects to the compositor named by $WAYLAND_DISPLAY and binds the
// globals it needs. Nothing is shown until Run is called.
func New(opts Options) (*Overlay, error) {
	if opts.Namespace == "" {
		opts.Namespace = "go-overlay"
	}
	o := &Overlay{opts: opts, outputs: map[uint32]*output{}, wakeR: -1, wakeW: -1}

	o.display = C.wl_display_connect(nil)
	if o.display == nil {
		return nil, fmt.Errorf("overlay: connect to Wayland display %q failed", os.Getenv("WAYLAND_DISPLAY"))
	}
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC|syscall.O_NONBLOCK); err != nil {
		o.Close()
		return nil, fmt.Errorf("overlay: create wake pipe: %w", err)
	}
	o.wakeR, o.wakeW = p[0], p[1]
	o.namespace = C.CString(opts.Namespace)

	o.handle = cgo.NewHandle(o)
	o.registry = C.wl_display_get_registry(o.display)
	C.ov_registry_add_listener(o.registry, C.uintptr_t(o.handle))
	// First roundtrip announces the globals, the second delivers the output
	// names and scales.
	for range 2 {
		if C.wl_display_roundtrip(o.display) < 0 {
			err := o.displayError()
			o.Close()
			return nil, err
		}
	}

	var err error
	switch {
	case o.compositor == nil:
		err = errors.New("overlay: compositor lacks wl_compositor v4")
	case o.shm == nil:
		err = errors.New("overlay: compositor lacks wl_shm")
	case o.layerShell == nil:
		err = errors.New("overlay: compositor does not support zwlr_layer_shell_v1")
	}
	if err != nil {
		o.Close()
		return nil, err
	}
	return o, nil
}

// Set shows l in corner c, replacing what was there. Safe to call from any
// goroutine, before or during Run.
func (o *Overlay) Set(c Corner, l Label) {
	cl := l.clone()
	o.mu.Lock()
	o.labels[c] = &cl
	o.gens[c]++
	o.mu.Unlock()
	o.wake()
}

// Clear hides corner c. Safe to call from any goroutine.
func (o *Overlay) Clear(c Corner) {
	o.mu.Lock()
	o.labels[c] = nil
	o.gens[c]++
	o.mu.Unlock()
	o.wake()
}

// Run shows the labels and processes compositor events until ctx is done
// (returning nil) or the connection fails. Only one Run may be active at a
// time; all labels are removed from the screen when it returns.
func (o *Overlay) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer o.removeAll()

	stop := context.AfterFunc(ctx, o.wake)
	defer stop()

	for ctx.Err() == nil {
		o.sync()
		if o.loopErr != nil {
			return o.loopErr
		}
		if C.ov_dispatch(o.display, C.int(o.wakeR)) < 0 {
			return o.displayError()
		}
	}
	return nil
}

// Close disconnects from the compositor. It must not be called while Run is
// active.
func (o *Overlay) Close() error {
	if o.display != nil {
		C.wl_display_disconnect(o.display)
		o.display = nil
	}
	for _, out := range o.outputs {
		out.forget()
	}
	o.outputs = nil
	if o.handle != 0 {
		o.handle.Delete()
		o.handle = 0
	}
	if o.namespace != nil {
		C.free(unsafe.Pointer(o.namespace))
		o.namespace = nil
	}
	for _, fd := range []*int{&o.wakeR, &o.wakeW} {
		if *fd >= 0 {
			syscall.Close(*fd)
			*fd = -1
		}
	}
	return nil
}

func (o *Overlay) wake() {
	syscall.Write(o.wakeW, []byte{0}) // EAGAIN means a wakeup is already pending
}

func (o *Overlay) fail(err error) {
	if o.loopErr == nil {
		o.loopErr = err
	}
}

func (o *Overlay) displayError() error {
	code := C.wl_display_get_error(o.display)
	if code == 0 {
		return errors.New("overlay: Wayland connection failed")
	}
	// libwayland already logged the details of protocol errors to stderr.
	return fmt.Errorf("overlay: Wayland connection: %w", syscall.Errno(code))
}

func (o *Overlay) bind(name uint32, iface *C.struct_wl_interface, version uint32) unsafe.Pointer {
	return C.wl_registry_bind(o.registry, C.uint32_t(name), iface, C.uint32_t(version))
}

func (o *Overlay) global(name uint32, iface string, version uint32) {
	switch iface {
	case "wl_compositor":
		if version >= 4 { // damage_buffer
			o.compositor = (*C.struct_wl_compositor)(o.bind(name, &C.wl_compositor_interface, min(version, 6)))
		}
	case "wl_shm":
		o.shm = (*C.struct_wl_shm)(o.bind(name, &C.wl_shm_interface, 1))
	case "zwlr_layer_shell_v1":
		o.layerShell = (*C.struct_zwlr_layer_shell_v1)(o.bind(name, &C.zwlr_layer_shell_v1_interface, min(version, 4)))
	case "wl_output":
		out := &output{o: o, global: name, version: min(version, 4)}
		out.wl = (*C.struct_wl_output)(o.bind(name, &C.wl_output_interface, out.version))
		out.handle = cgo.NewHandle(out)
		C.ov_output_add_listener(out.wl, C.uintptr_t(out.handle))
		o.outputs[name] = out
	}
}

func (o *Overlay) globalRemove(name uint32) {
	if out, ok := o.outputs[name]; ok {
		out.destroy()
		delete(o.outputs, name)
	}
}

func (o *Overlay) wants(out *output) bool {
	return len(o.opts.Outputs) == 0 || slices.Contains(o.opts.Outputs, out.name)
}

// sync brings the surfaces of every output in line with the requested labels.
func (o *Overlay) sync() {
	o.mu.Lock()
	labels, gens := o.labels, o.gens
	o.mu.Unlock()

	for _, out := range o.outputs {
		if !out.ready || !o.wants(out) {
			continue
		}
		for c := range out.surfaces {
			s := out.surfaces[c]
			switch {
			case labels[c] == nil:
				if s != nil {
					s.destroy()
					out.surfaces[c] = nil
				}
			case s == nil:
				if out.closedGen[c] != gens[c] {
					out.surfaces[c] = newSurface(out, Corner(c), labels[c], gens[c])
				}
			case s.gen != gens[c]:
				s.setLabel(labels[c], gens[c])
			}
		}
	}
}

// removeAll destroys every surface and waits until the compositor has seen it.
func (o *Overlay) removeAll() {
	for _, out := range o.outputs {
		out.destroySurfaces()
	}
	C.wl_display_roundtrip(o.display)
}

// output is one monitor.
type output struct {
	o       *Overlay
	global  uint32
	version uint32
	wl      *C.struct_wl_output
	handle  cgo.Handle

	name         string
	scale        int
	pendingScale int
	ready        bool // first done event received

	surfaces  [4]*surface
	closedGen [4]uint64 // label generation the compositor closed; not recreated until it changes
}

func (out *output) done() {
	out.ready = true
	if out.pendingScale > 0 && out.pendingScale != out.scale {
		out.scale = out.pendingScale
		for _, s := range out.surfaces {
			if s != nil && s.preferredScale == 0 {
				s.draw()
			}
		}
	}
}

func (out *output) destroySurfaces() {
	for c, s := range out.surfaces {
		if s != nil {
			s.destroy()
			out.surfaces[c] = nil
		}
	}
}

func (out *output) destroy() {
	out.destroySurfaces()
	if out.version >= 3 {
		C.wl_output_release(out.wl)
	} else {
		C.wl_output_destroy(out.wl)
	}
	out.forget()
}

// forget drops Go-side resources without talking to the compositor.
func (out *output) forget() {
	if out.handle != 0 {
		out.handle.Delete()
		out.handle = 0
	}
}
