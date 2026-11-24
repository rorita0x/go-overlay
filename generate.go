package overlay

// The generated protocol sources are committed, so building only needs the
// system libraries. Regenerate with `go generate` after updating protocols/.

//go:generate wayland-scanner client-header protocols/wlr-layer-shell-unstable-v1.xml wlr-layer-shell-unstable-v1-client-protocol.h
//go:generate wayland-scanner private-code protocols/wlr-layer-shell-unstable-v1.xml wlr-layer-shell-unstable-v1-protocol.c
//go:generate wayland-scanner client-header protocols/xdg-shell.xml xdg-shell-client-protocol.h
//go:generate wayland-scanner private-code protocols/xdg-shell.xml xdg-shell-protocol.c
//go:generate wayland-scanner client-header protocols/cursor-shape-v1.xml cursor-shape-v1-client-protocol.h
//go:generate wayland-scanner private-code protocols/cursor-shape-v1.xml cursor-shape-v1-protocol.c
// cursor-shape-v1 refers to zwp_tablet_tool_v2, so tablet-v2 only needs its interfaces.
//go:generate wayland-scanner private-code protocols/tablet-v2.xml tablet-v2-protocol.c
