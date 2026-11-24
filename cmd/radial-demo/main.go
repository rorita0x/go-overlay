// radial-demo opens a radial menu where the mouse pointer is and prints
// what is picked. A click on the label in the top right corner opens it
// again (or closes it), and so does SIGUSR1, e.g. from a desktop shortcut
// running `pkill -USR1 radial-demo`. Ctrl+C quits.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	overlay "github.com/rorita0x/go-overlay"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ov, err := overlay.New(overlay.Options{Namespace: "radial-demo"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "radial-demo:", err)
		os.Exit(1)
	}
	defer ov.Close()

	toggle := func() {
		if ov.RadialOpen() {
			ov.ClearRadial()
		} else {
			ov.SetRadial(menu())
		}
	}
	ov.Set(overlay.TopRight, overlay.Label{
		Spans:   []overlay.Span{{Text: "◎ radial menu"}},
		Style:   overlay.DefaultTextStyle,
		OnClick: func(overlay.Click) { toggle() },
	})
	usr1 := make(chan os.Signal, 1)
	signal.Notify(usr1, syscall.SIGUSR1)
	go func() {
		for range usr1 {
			toggle()
		}
	}()

	ov.SetRadial(menu())
	if err := ov.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "radial-demo:", err)
		os.Exit(1)
	}
}

func menu() overlay.Radial {
	accent := overlay.DefaultRadialStyle.Accent
	muted := overlay.DefaultRadialStyle.Muted
	return overlay.Radial{
		Items: []overlay.RadialItem{
			{Label: "Hello", Value: "hello"},
			{Label: "Clock", Value: "clock", Preview: func() []overlay.Span {
				return []overlay.Span{{Text: "Clock\n", Bold: true}, {Text: time.Now().Format("15:04:05")}}
			}},
			{Label: "Colors", Detail: "4 entries ›", Children: []overlay.RadialItem{
				{Label: "Red"}, {Label: "Green"}, {Label: "Blue"}, {Label: "Yellow"},
			}},
			{Label: "Shapes", Detail: "3 entries ›", Children: []overlay.RadialItem{
				{Label: "Circle"}, {Label: "Square"},
				{Label: "More", Detail: "2 entries ›", Children: []overlay.RadialItem{{Label: "Star"}, {Label: "Heart"}}},
			}},
			{Label: "Locked", Disabled: true, Preview: func() []overlay.Span {
				return []overlay.Span{{Text: "Locked\n", Bold: true}, {Text: "not available", Color: muted}}
			}},
			{Label: ""}, // a blank slot keeps the other directions where they are
			{Label: "Numbers", Detail: "3 entries ›", Children: []overlay.RadialItem{
				{Label: "One"}, {Label: "Two"}, {Label: "Three"},
			}},
			{Label: "Bye"},
		},
		Center: overlay.RadialItem{Value: "center", Preview: func() []overlay.Span {
			return []overlay.Span{{Text: "radial-demo\n", Bold: true}, {Text: "click: center", Color: accent}}
		}},
		Refresh: time.Second, // keeps the clock preview running
		OnPick: func(p overlay.RadialPick) {
			fmt.Printf("picked %q (path %v, center %v) with the %s button\n", p.Value, p.Path, p.Center, buttonName(p.Button))
		},
		OnCancel: func() { fmt.Println("cancelled") },
	}
}

func buttonName(b overlay.Button) string {
	switch b {
	case overlay.ButtonLeft:
		return "left"
	case overlay.ButtonRight:
		return "right"
	case overlay.ButtonMiddle:
		return "middle"
	}
	return fmt.Sprintf("%#x", uint32(b))
}
