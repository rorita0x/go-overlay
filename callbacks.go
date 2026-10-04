package overlay

// #include <stdint.h>
import "C"

import "runtime/cgo"

// Event handlers called from the listeners in shim.c. They run on the thread
// that dispatches the display, i.e. inside Run or New.

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

//export ovSurfacePreferredScale
func ovSurfacePreferredScale(h C.uintptr_t, factor C.int32_t) {
	cgo.Handle(h).Value().(*surface).setPreferredScale(int(factor))
}

//export ovLayerConfigure
func ovLayerConfigure(h C.uintptr_t, serial, w, hgt C.uint32_t) {
	cgo.Handle(h).Value().(*surface).configure(uint32(serial))
}

//export ovLayerClosed
func ovLayerClosed(h C.uintptr_t) {
	cgo.Handle(h).Value().(*surface).closed()
}

//export ovBufferRelease
func ovBufferRelease(h C.uintptr_t) {
	cgo.Handle(h).Value().(*buffer).release()
}
