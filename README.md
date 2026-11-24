# go-overlay

Go library that draws text labels in the four screen corners on Wayland,
click-through unless they take clicks, and a radial menu at the mouse
pointer, like a tiny LayerShellQt for Go. It uses the `wlr-layer-shell`
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

## Images

`SetImage` shows a picture you rendered yourself, e.g. with a software
renderer. The pixels are copied, so the `*image.RGBA` can be reused for the
next frame. `Overlay.Scale()` returns the highest buffer scale of the outputs
in use; render at that many pixels per logical pixel and pass it as `Scale`.
`Opacity` fades the whole picture, e.g. `0.5` for half transparent. Leaving it
unset (0) or setting it to 1 shows the picture fully opaque.

```go
sc := ov.Scale()
img := image.NewRGBA(image.Rect(0, 0, 300*sc, 400*sc))
draw(img) // premultiplied RGBA
ov.SetImage(overlay.TopRight, overlay.Image{RGBA: img, Scale: sc, Margin: 12})
```

## Clickable labels

A label with `OnClick` takes mouse clicks; every other label stays
click-through. `OnClick` is called when a button is released over the label
it was pressed on, with the button and the position in the label. Clicking a
label never takes the keyboard focus from the window that has it.

```go
ov.Set(overlay.TopRight, overlay.Label{
	Spans:   []overlay.Span{{Text: "◎ menu"}},
	Style:   overlay.DefaultTextStyle,
	OnClick: func(c overlay.Click) { ov.SetRadial(menu) },
})
```

## Radial menu

`SetRadial` opens a radial (pie) menu centred where the mouse pointer is
(moved inwards if it would not fit on the screen). Its items fill the ring
clockwise from the top; an item with an empty `Label` leaves its slot blank,
so the others keep their directions, and a `Disabled` one is drawn dimmed.

- Hovering a slot shows its `Preview` (or its label) in the middle. While
  nothing is hovered the middle shows the menu's `Center`, and clicking it
  picks `Center` (`RadialPick.Center`). `Refresh` redraws the open menu
  periodically, e.g. for a countdown in a preview.
- An item with `Children` opens a submenu: a new ring centred where the item
  was clicked, so the pointer starts in its middle. A click on a submenu's
  middle goes back to the ring before, at its old place.
- A click on an item (any mouse button) closes the menu and calls `OnPick`
  with the item's path, its `Value` (or label) and the button. A click
  outside the ring closes it and calls `OnCancel`. `ClearRadial` closes it
  without a callback; `RadialOpen` tells whether it is open.

While open, the menu takes the pointer on the whole screen (one transparent
surface per output) but never the keyboard focus, so the window that had it
keeps it: an app can type into it after a pick. For the same reason there is
no Escape; a click beside the ring closes the menu.

```go
ov.SetRadial(overlay.Radial{
	Items: []overlay.RadialItem{
		{Label: "Copy", Value: "copy"},
		{Label: "Colors", Detail: "3 entries ›", Children: []overlay.RadialItem{
			{Label: "Red"}, {Label: "Green"}, {Label: "Blue"},
		}},
		{Label: "Paste", Disabled: true},
	},
	Center:   overlay.RadialItem{Label: "Close", Value: "close"},
	OnPick:   func(p overlay.RadialPick) { fmt.Println(p.Value, p.Path, p.Button) },
	OnCancel: func() { fmt.Println("cancelled") },
})
```

## Build requirements

cgo, plus development files for `wayland-client`, `pangocairo` and `xkbcommon`.
The protocol glue in `*-protocol.c/h` is generated and committed; `go generate`
regenerates it from `protocols/` (needs `wayland-scanner`).

## Demo

```sh
go run ./cmd/overlay-demo                 # all outputs
go run ./cmd/overlay-demo -list-outputs   # print monitor names
go run ./cmd/overlay-demo -outputs DP-1   # one output
go run ./cmd/overlay-demo -over-panels
go run ./cmd/overlay-demo -image          # animated gradient top left
ls --color=always | go run ./cmd/overlay-demo -stdin   # last 10 lines, bottom left
go run ./cmd/radial-demo                  # a radial menu; reopen via the corner label or `pkill -USR1 radial-demo`
```

## Limitations

The pointer image over clickable labels and the radial menu is set through
`cursor-shape-v1` (KWin 6, Sway, Hyprland, niri, …). A compositor without
it may hide the pointer over them, i.e. on the whole screen while a radial
menu is open.

Integer buffer scale only (`preferred_buffer_scale` / `wl_output.scale`); with
fractional scaling the compositor scales the buffer.

## License

Public domain (Unlicense), see [LICENSE](LICENSE). `protocols/` and the generated `*-protocol.c/h` files keep
their upstream licenses (stated in each file).
