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
named outputs (`DP-1`, …); `overlay.ListOutputs()` returns the names of the
connected monitors. `Options.ExclusiveZone = -1` places labels on top of panels.

## Plain text with terminal colors

`SetText` takes a string and colors it the way a terminal would, from ANSI SGR
escape codes (16/256/truecolor, bold, italic, underline, reverse, …). All other
escape sequences are stripped. It uses `Options.TextStyle` (default
`DefaultTextStyle`) and `Options.Palette` (default xterm colors).

```go
out, _ := exec.Command("git", "-c", "color.status=always", "status", "-s").Output()
ov.SetText(overlay.BottomLeft, string(out))
```

`ParseANSI(text, palette)` returns the spans, so you can combine them with
your own `Style` via `Set`.

## Build requirements

cgo, plus development files for `wayland-client` and `pangocairo`.
The protocol glue in `*-protocol.c/h` is generated and committed; `go generate`
regenerates it from `protocols/` (needs `wayland-scanner`).

## Demo

```sh
go run ./cmd/overlay-demo                 # all outputs
go run ./cmd/overlay-demo -list-outputs   # print monitor names
go run ./cmd/overlay-demo -outputs DP-1   # one output
go run ./cmd/overlay-demo -over-panels
ls --color=always | go run ./cmd/overlay-demo -stdin   # last 10 lines, bottom left
```

## Limitations

Integer buffer scale only (`preferred_buffer_scale` / `wl_output.scale`); with
fractional scaling the compositor scales the buffer.

## License

Public domain (Unlicense), see [LICENSE](LICENSE). `protocols/` and the generated `*-protocol.c/h` files keep
their upstream licenses (stated in each file).
