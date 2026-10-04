// Command overlay-demo shows labels in all four screen corners, including a
// live clock. Stop it with Ctrl-C.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	overlay "github.com/rorita0x/go-overlay"
)

func main() {
	outputs := flag.String("outputs", "", "comma-separated output names (default: all)")
	onPanels := flag.Bool("over-panels", false, "draw on top of panels instead of next to them")
	flag.Parse()

	opts := overlay.Options{}
	if *outputs != "" {
		opts.Outputs = strings.Split(*outputs, ",")
	}
	if *onPanels {
		opts.ExclusiveZone = -1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ov, err := overlay.New(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer ov.Close()

	var (
		box = overlay.Style{
			Font:       "Monospace 11",
			Background: overlay.MustHex("#1e1e2ecc"),
			Padding:    8,
			Radius:     8,
			Margin:     12,
		}
		grey   = overlay.MustHex("#9399b2")
		green  = overlay.MustHex("#a6e3a1")
		yellow = overlay.MustHex("#f9e2af")
		red    = overlay.MustHex("#f38ba8")
		blue   = overlay.MustHex("#89b4fa")
	)

	ov.Set(overlay.TopLeft, overlay.Label{Style: box, Spans: []overlay.Span{
		{Text: "● ", Color: green},
		{Text: "go-overlay", Bold: true},
		{Text: " on ", Color: grey},
		{Text: os.Getenv("XDG_CURRENT_DESKTOP"), Color: blue, Italic: true},
	}})

	alert := box
	alert.Font = "Sans Bold 14"
	alert.Background = overlay.MustHex("#00000080")
	ov.Set(overlay.BottomLeft, overlay.Label{Style: alert, Spans: []overlay.Span{
		{Text: "REC", Color: red},
	}})

	ov.Set(overlay.BottomRight, overlay.Label{Style: box, Spans: []overlay.Span{
		{Text: "cpu ", Color: grey}, {Text: "12%\n", Color: green},
		{Text: "mem ", Color: grey}, {Text: "61%\n", Color: yellow},
		{Text: "disk ", Color: grey}, {Text: "93%", Color: red, Bold: true},
	}})

	clock := func(t time.Time) overlay.Label {
		return overlay.Label{Style: box, Spans: []overlay.Span{
			{Text: t.Format("Mon 02 Jan "), Color: grey},
			{Text: t.Format("15:04:05"), Bold: true},
		}}
	}
	ov.Set(overlay.TopRight, clock(time.Now()))
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-tick.C:
				ov.Set(overlay.TopRight, clock(t))
			}
		}
	}()

	if err := ov.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
