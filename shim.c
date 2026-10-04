#define _GNU_SOURCE
#include <errno.h>
#include <poll.h>
#include <sys/mman.h>
#include <unistd.h>

#include "shim.h"
#include "_cgo_export.h"

static void registry_global(void *data, struct wl_registry *r, uint32_t name,
                            const char *iface, uint32_t version) {
	ovRegistryGlobal((uintptr_t)data, name, (char *)iface, version);
}

static void registry_global_remove(void *data, struct wl_registry *r, uint32_t name) {
	ovRegistryGlobalRemove((uintptr_t)data, name);
}

static const struct wl_registry_listener registry_listener = {
	.global = registry_global,
	.global_remove = registry_global_remove,
};

void ov_registry_add_listener(struct wl_registry *r, uintptr_t h) {
	wl_registry_add_listener(r, &registry_listener, (void *)h);
}

static void output_geometry(void *data, struct wl_output *o, int32_t x, int32_t y,
                            int32_t pw, int32_t ph, int32_t subpixel, const char *make,
                            const char *model, int32_t transform) {}
static void output_mode(void *data, struct wl_output *o, uint32_t flags, int32_t w,
                        int32_t h, int32_t refresh) {}
static void output_done(void *data, struct wl_output *o) { ovOutputDone((uintptr_t)data); }
static void output_scale(void *data, struct wl_output *o, int32_t factor) {
	ovOutputScale((uintptr_t)data, factor);
}
static void output_name(void *data, struct wl_output *o, const char *name) {
	ovOutputName((uintptr_t)data, (char *)name);
}
static void output_description(void *data, struct wl_output *o, const char *desc) {}

static const struct wl_output_listener output_listener = {
	.geometry = output_geometry,
	.mode = output_mode,
	.done = output_done,
	.scale = output_scale,
	.name = output_name,
	.description = output_description,
};

void ov_output_add_listener(struct wl_output *o, uintptr_t h) {
	wl_output_add_listener(o, &output_listener, (void *)h);
}

static void surface_enter(void *data, struct wl_surface *s, struct wl_output *o) {}
static void surface_leave(void *data, struct wl_surface *s, struct wl_output *o) {}
static void surface_preferred_buffer_scale(void *data, struct wl_surface *s, int32_t factor) {
	ovSurfacePreferredScale((uintptr_t)data, factor);
}
static void surface_preferred_buffer_transform(void *data, struct wl_surface *s, uint32_t t) {}

static const struct wl_surface_listener surface_listener = {
	.enter = surface_enter,
	.leave = surface_leave,
	.preferred_buffer_scale = surface_preferred_buffer_scale,
	.preferred_buffer_transform = surface_preferred_buffer_transform,
};

void ov_surface_add_listener(struct wl_surface *s, uintptr_t h) {
	wl_surface_add_listener(s, &surface_listener, (void *)h);
}

static void layer_configure(void *data, struct zwlr_layer_surface_v1 *l, uint32_t serial,
                            uint32_t w, uint32_t h) {
	ovLayerConfigure((uintptr_t)data, serial, w, h);
}
static void layer_closed(void *data, struct zwlr_layer_surface_v1 *l) {
	ovLayerClosed((uintptr_t)data);
}

static const struct zwlr_layer_surface_v1_listener layer_listener = {
	.configure = layer_configure,
	.closed = layer_closed,
};

void ov_layer_surface_add_listener(struct zwlr_layer_surface_v1 *l, uintptr_t h) {
	zwlr_layer_surface_v1_add_listener(l, &layer_listener, (void *)h);
}

static void buffer_release(void *data, struct wl_buffer *b) { ovBufferRelease((uintptr_t)data); }

static const struct wl_buffer_listener buffer_listener = {
	.release = buffer_release,
};

void ov_buffer_add_listener(struct wl_buffer *b, uintptr_t h) {
	wl_buffer_add_listener(b, &buffer_listener, (void *)h);
}

int ov_shm_alloc(size_t size, void **data) {
	int fd = memfd_create("go-overlay", MFD_CLOEXEC);
	if (fd < 0) return -1;
	if (ftruncate(fd, size) < 0) goto fail;
	void *p = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (p == MAP_FAILED) goto fail;
	*data = p;
	return fd;
fail:;
	int e = errno;
	close(fd);
	errno = e;
	return -1;
}

void ov_shm_free(void *data, size_t size) { munmap(data, size); }

int ov_dispatch(struct wl_display *d, int wakefd) {
	while (wl_display_prepare_read(d) != 0) {
		if (wl_display_dispatch_pending(d) < 0) return -1;
	}

	int fd = wl_display_get_fd(d);
	while (wl_display_flush(d) < 0) {
		if (errno != EAGAIN) {
			wl_display_cancel_read(d);
			return -1;
		}
		struct pollfd out = {.fd = fd, .events = POLLOUT};
		poll(&out, 1, -1);
	}

	struct pollfd fds[2] = {
		{.fd = fd, .events = POLLIN},
		{.fd = wakefd, .events = POLLIN},
	};
	if (poll(fds, 2, -1) < 0) {
		wl_display_cancel_read(d);
		return errno == EINTR ? 0 : -1;
	}

	if (fds[0].revents & (POLLIN | POLLERR | POLLHUP)) {
		if (wl_display_read_events(d) < 0) return -1;
	} else {
		wl_display_cancel_read(d);
	}

	if (fds[1].revents & POLLIN) {
		char buf[64];
		while (read(wakefd, buf, sizeof buf) > 0) {
		}
	}
	return wl_display_dispatch_pending(d) < 0 ? -1 : 0;
}
