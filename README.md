# go-overlay

Go library that draws click-through text labels in the four screen corners on
Wayland, like a tiny LayerShellQt for Go. It uses the `wlr-layer-shell`
protocol (KWin, Sway, Hyprland, niri, …) and renders with Pango/Cairo, so you
get system fonts, shaping, emoji, multi-colored spans and a rounded background box.

```go
ov, err := overlay.New(overlay.Options{}) // all outputs
if err != nil { ... }
defer ov.Close()

ov.Set(overlay.TopRight, overlay.Label{
	Style: overlay.Style{
		Font:       "Monospace 11",
		Background: overlay.MustHex("#1e1e2ecc"),
		Padding:    8, Radius: 8, Margin: 12,
	},
	Spans: []overlay.Span{
		{Text: "cpu ", Color: overlay.MustHex("#9399b2")},
		{Text: "93%", Color: overlay.MustHex("#f38ba8"), Bold: true},
	},
})

err = ov.Run(ctx) // blocks; Set/Clear may be called from any goroutine
```

Colors are `color.NRGBA` (straight alpha). `Options.Outputs` limits the overlay to
named outputs (`DP-1`, …), and `Options.ExclusiveZone = -1` places labels on top of panels.

## Build requirements

cgo, plus development files for `wayland-client` and `pangocairo`.
The protocol glue in `*-protocol.c/h` is generated and committed; `go generate`
regenerates it from `protocols/` (needs `wayland-scanner`).

## Demo

```sh
go run ./cmd/overlay-demo                 # all outputs
go run ./cmd/overlay-demo -outputs DP-1   # one output
go run ./cmd/overlay-demo -over-panels
```

## Limitations

Integer buffer scale only (`preferred_buffer_scale` / `wl_output.scale`); with
fractional scaling the compositor scales the buffer.
