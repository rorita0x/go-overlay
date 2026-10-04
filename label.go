package overlay

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// Corner selects one of the four screen corners.
type Corner int

const (
	TopLeft Corner = iota
	TopRight
	BottomLeft
	BottomRight
)

func (c Corner) left() bool { return c == TopLeft || c == BottomLeft }
func (c Corner) top() bool  { return c == TopLeft || c == TopRight }

// Align is the horizontal alignment of lines inside a label.
type Align int

const (
	// AlignAuto aligns left in the left corners and right in the right corners.
	AlignAuto Align = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Span is a run of text sharing one color and font style.
type Span struct {
	Text          string      // may contain "\n" to start a new line
	Color         color.NRGBA // zero value: Style.Foreground
	Background    color.NRGBA // highlight behind this span; A == 0 draws none
	Bold          bool
	Italic        bool
	Underline     bool
	Strikethrough bool
}

// Style controls the font and the box drawn behind the text.
type Style struct {
	Font       string      // Pango font description, e.g. "JetBrains Mono 11"; default "Sans 11"
	Foreground color.NRGBA // used for spans with a zero Color; default opaque white
	Background color.NRGBA // box behind the text; A == 0 draws no box
	Padding    int         // space between box edge and text, in logical pixels
	Radius     float64     // corner radius of the box
	Margin     int         // distance from the screen edges
	Align      Align
}

// Label is the content of one corner.
type Label struct {
	Spans []Span
	Style Style
}

var white = color.NRGBA{255, 255, 255, 255}

// DefaultTextStyle is used by SetText when Options.TextStyle is not set.
var DefaultTextStyle = Style{
	Font:       "Monospace 11",
	Background: color.NRGBA{0x1e, 0x1e, 0x2e, 0xcc},
	Padding:    8,
	Radius:     8,
	Margin:     12,
}

// clone returns a deep copy with defaults filled in, so later changes by the
// caller don't race with the event loop.
func (l Label) clone() Label {
	l.Spans = append([]Span(nil), l.Spans...)
	if l.Style.Font == "" {
		l.Style.Font = "Sans 11"
	}
	if l.Style.Foreground.A == 0 {
		l.Style.Foreground = white
	}
	return l
}

// Hex parses "#rgb", "#rgba", "#rrggbb" or "#rrggbbaa" (the "#" is optional).
func Hex(s string) (color.NRGBA, error) {
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 || len(h) == 4 {
		var b strings.Builder
		for _, r := range h {
			b.WriteRune(r)
			b.WriteRune(r)
		}
		h = b.String()
	}
	if len(h) == 6 {
		h += "ff"
	}
	if len(h) != 8 {
		return color.NRGBA{}, fmt.Errorf("overlay: invalid hex color %q", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("overlay: invalid hex color %q: %w", s, err)
	}
	return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

// MustHex is like Hex but panics on invalid input. Meant for constants.
func MustHex(s string) color.NRGBA {
	c, err := Hex(s)
	if err != nil {
		panic(err)
	}
	return c
}
