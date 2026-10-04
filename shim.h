#ifndef GO_OVERLAY_SHIM_H
#define GO_OVERLAY_SHIM_H

#include <stddef.h>
#include <stdint.h>
#include <wayland-client.h>
#include "wlr-layer-shell-unstable-v1-client-protocol.h"

// Listeners forward events to exported Go functions. The user data of every
// object is a runtime/cgo.Handle to its Go counterpart.
void ov_registry_add_listener(struct wl_registry *r, uintptr_t h);
void ov_output_add_listener(struct wl_output *o, uintptr_t h);
void ov_surface_add_listener(struct wl_surface *s, uintptr_t h);
void ov_layer_surface_add_listener(struct zwlr_layer_surface_v1 *l, uintptr_t h);
void ov_buffer_add_listener(struct wl_buffer *b, uintptr_t h);

// ov_shm_alloc creates a memfd of size bytes and maps it. Returns the fd, or
// -1 with errno set.
int ov_shm_alloc(size_t size, void **data);
void ov_shm_free(void *data, size_t size);

// ov_dispatch flushes requests, waits until the display or wakefd is readable
// and dispatches the events that arrived. Returns -1 on a connection error.
int ov_dispatch(struct wl_display *d, int wakefd);

#endif
