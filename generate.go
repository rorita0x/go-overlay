package overlay

// The generated protocol sources are committed, so building only needs the
// system libraries. Regenerate with `go generate` after updating protocols/.

//go:generate wayland-scanner client-header protocols/wlr-layer-shell-unstable-v1.xml wlr-layer-shell-unstable-v1-client-protocol.h
//go:generate wayland-scanner private-code protocols/wlr-layer-shell-unstable-v1.xml wlr-layer-shell-unstable-v1-protocol.c
//go:generate wayland-scanner client-header protocols/xdg-shell.xml xdg-shell-client-protocol.h
//go:generate wayland-scanner private-code protocols/xdg-shell.xml xdg-shell-protocol.c
