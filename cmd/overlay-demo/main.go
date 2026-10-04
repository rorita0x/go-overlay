// Command overlay-demo shows labels in all four screen corners, including a
// live clock. Stop it with Ctrl-C.
package main

import (
	"bufio"
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
	stdin := flag.Bool("stdin", false, "show the last 10 lines of stdin (ANSI colors allowed) bottom left")
	listOutputs := flag.Bool("list-outputs", false, "print the connected outputs and exit")
	flag.Parse()

	if *listOutputs {
		list, err := overlay.ListOutputs()
		if err != nil {
			log.Fatal(err)
		}
		for _, out := range list {
			fmt.Printf("%-10s scale %d  %s\n", out.Name, out.Scale, out.Description)
		}
		return
	}

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
		grey  = overlay.MustHex("#9399b2")
		green = overlay.MustHex("#a6e3a1")
		red   = overlay.MustHex("#f38ba8")
		blue  = overlay.MustHex("#89b4fa")
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

	// Plain text with terminal escape codes, styled by Options.TextStyle.
	ov.SetText(overlay.BottomRight, "\x1b[90mcpu \x1b[32m12%\n"+
		"\x1b[90mmem \x1b[33m61%\n"+
		"\x1b[90mdisk \x1b[1;31m93%\x1b[0m\n"+
		"\x1b[38;5;213m256\x1b[0m \x1b[38;2;137;180;250mtrue\x1b[0m \x1b[4munder\x1b[0m \x1b[7mrev\x1b[0m")

	if *stdin {
		go showStdin(ov)
	}

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

// showStdin displays the last lines read from stdin, updating on every line.
func showStdin(ov *overlay.Overlay) {
	var lines []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > 10 {
			lines = lines[1:]
		}
		ov.SetText(overlay.BottomLeft, strings.Join(lines, "\n"))
	}
}
