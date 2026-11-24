package overlay

// #include <stdint.h>
import "C"

import "runtime/cgo"

// Event handlers called from the listeners in shim.c. They run on the thread
// that dispatches the display, i.e. inside Run or New.

// scaler receives wl_surface.preferred_buffer_scale. Satisfied by *surface
// and *popup.
type scaler interface{ setPreferredScale(int) }

// layerSurface receives zwlr_layer_surface_v1 configure/closed. Satisfied by
// *surface and *popup. w and h are the size the compositor picked, 0 where
// it leaves it to the client.
type layerSurface interface {
	configure(serial, w, h uint32)
	closed()
}

// releaser receives wl_buffer.release. Satisfied by *buffer and *shmBuffer.
type releaser interface{ release() }

//export ovRegistryGlobal
func ovRegistryGlobal(h C.uintptr_t, name C.uint32_t, iface *C.char, version C.uint32_t) {
	cgo.Handle(h).Value().(*Overlay).global(uint32(name), C.GoString(iface), uint32(version))
}

//export ovRegistryGlobalRemove
func ovRegistryGlobalRemove(h C.uintptr_t, name C.uint32_t) {
	cgo.Handle(h).Value().(*Overlay).globalRemove(uint32(name))
}

//export ovOutputDone
func ovOutputDone(h C.uintptr_t) {
	cgo.Handle(h).Value().(*output).done()
}

//export ovOutputScale
func ovOutputScale(h C.uintptr_t, factor C.int32_t) {
	cgo.Handle(h).Value().(*output).pendingScale = int(factor)
}

//export ovOutputName
func ovOutputName(h C.uintptr_t, name *C.char) {
	cgo.Handle(h).Value().(*output).name = C.GoString(name)
}

//export ovOutputDescription
func ovOutputDescription(h C.uintptr_t, desc *C.char) {
	cgo.Handle(h).Value().(*output).description = C.GoString(desc)
}

//export ovSurfacePreferredScale
func ovSurfacePreferredScale(h C.uintptr_t, factor C.int32_t) {
	cgo.Handle(h).Value().(scaler).setPreferredScale(int(factor))
}

//export ovLayerConfigure
func ovLayerConfigure(h C.uintptr_t, serial, w, hgt C.uint32_t) {
	cgo.Handle(h).Value().(layerSurface).configure(uint32(serial), uint32(w), uint32(hgt))
}

//export ovLayerClosed
func ovLayerClosed(h C.uintptr_t) {
	cgo.Handle(h).Value().(layerSurface).closed()
}

//export ovBufferRelease
func ovBufferRelease(h C.uintptr_t) {
	cgo.Handle(h).Value().(releaser).release()
}
