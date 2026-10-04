package overlay

import (
	"image/color"
	"strconv"
	"strings"
)

// Palette holds the 16 ANSI colors: 0–7 normal, 8–15 bright.
type Palette [16]color.NRGBA

// DefaultPalette is the standard xterm palette.
var DefaultPalette = Palette{
	{0x00, 0x00, 0x00, 0xff}, {0xcd, 0x00, 0x00, 0xff}, {0x00, 0xcd, 0x00, 0xff}, {0xcd, 0xcd, 0x00, 0xff},
	{0x00, 0x00, 0xee, 0xff}, {0xcd, 0x00, 0xcd, 0xff}, {0x00, 0xcd, 0xcd, 0xff}, {0xe5, 0xe5, 0xe5, 0xff},
	{0x7f, 0x7f, 0x7f, 0xff}, {0xff, 0x00, 0x00, 0xff}, {0x00, 0xff, 0x00, 0xff}, {0xff, 0xff, 0x00, 0xff},
	{0x5c, 0x5c, 0xff, 0xff}, {0xff, 0x00, 0xff, 0xff}, {0x00, 0xff, 0xff, 0xff}, {0xff, 0xff, 0xff, 0xff},
}

var black = color.NRGBA{0, 0, 0, 255}

// ParseANSI converts text containing ANSI escape sequences into spans, the
// way a terminal would color it. SGR sequences (colors, bold, italic,
// underline, …) are applied; every other escape sequence and control
// character except "\n" and "\t" is dropped, as are trailing newlines.
// A nil palette means DefaultPalette.
//
// Spans using the default foreground keep a zero Color, so they render in
// Style.Foreground. Reverse video and dim with the default foreground assume
// white text on black.
func ParseANSI(text string, p *Palette) []Span {
	if p == nil {
		p = &DefaultPalette
	}
	var (
		spans []Span
		buf   strings.Builder
		st    sgrState
	)
	flush := func() {
		if buf.Len() > 0 {
			spans = append(spans, st.span(buf.String()))
			buf.Reset()
		}
	}
	for i := 0; i < len(text); {
		switch ch := text[i]; {
		case ch == 0x1b:
			n, params, final := scanEscape(text[i:])
			if final == 'm' && (params == "" || params[0] < '<') { // '<'..'?' prefixes are private modes
				flush()
				st.apply(params, p)
			}
			i += n
		case ch == '\n' || ch == '\t':
			buf.WriteByte(ch)
			i++
		case ch < 0x20 || ch == 0x7f:
			i++
		case ch == 0xc2 && i+1 < len(text) && text[i+1] >= 0x80 && text[i+1] <= 0x9f:
			i += 2 // C1 control characters (U+0080–U+009F)
		default:
			buf.WriteByte(ch)
			i++
		}
	}
	flush()

	for len(spans) > 0 {
		last := &spans[len(spans)-1]
		last.Text = strings.TrimRight(last.Text, "\n")
		if last.Text != "" {
			break
		}
		spans = spans[:len(spans)-1]
	}
	return spans
}

// scanEscape parses the escape sequence at the start of s (s[0] == ESC) and
// returns its length. For CSI sequences it also returns the parameter bytes and
// the final byte; final is 0 for anything else and for malformed sequences.
func scanEscape(s string) (n int, params string, final byte) {
	if len(s) < 2 {
		return len(s), "", 0
	}
	switch s[1] {
	case '[':
		j := 2
		for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3f {
			j++
		}
		params = s[2:j]
		for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2f {
			j++
		}
		if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7e {
			return j + 1, params, s[j]
		}
		// Malformed: like a terminal, ignore everything up to the final byte.
		for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3f {
			j++
		}
		if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7e {
			j++
		}
		return j, "", 0
	case ']', 'P', 'X', '^', '_':
		// OSC, DCS, SOS, PM, APC: run until BEL or ST (ESC \).
		for j := 2; j < len(s); j++ {
			if s[j] == 0x07 {
				return j + 1, "", 0
			}
			if s[j] == 0x1b {
				if j+1 < len(s) && s[j+1] == '\\' {
					return j + 2, "", 0
				}
				return j, "", 0 // unterminated; let the next ESC start over
			}
		}
		return len(s), "", 0
	default:
		// ESC, intermediate bytes, one final byte (e.g. "ESC ( B").
		j := 1
		for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2f {
			j++
		}
		if j < len(s) && s[j] >= 0x30 && s[j] <= 0x7e {
			j++
		}
		return j, "", 0
	}
}

type sgrState struct {
	fg, bg                                               color.NRGBA // A == 0: default
	bold, dim, italic, underline, strikethrough, reverse bool
}

func (st *sgrState) span(text string) Span {
	fg, bg := st.fg, st.bg
	if st.reverse {
		if fg.A == 0 {
			fg = white
		}
		if bg.A == 0 {
			bg = black
		}
		fg, bg = bg, fg
	}
	if st.dim {
		if fg.A == 0 {
			fg = white
		}
		fg.A /= 2
	}
	return Span{
		Text:          text,
		Color:         fg,
		Background:    bg,
		Bold:          st.bold,
		Italic:        st.italic,
		Underline:     st.underline,
		Strikethrough: st.strikethrough,
	}
}

func (st *sgrState) apply(params string, p *Palette) {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if strings.Contains(part, ":") {
			st.applySub(strings.Split(part, ":"), p)
			continue
		}
		code := 0
		if part != "" {
			var err error
			if code, err = strconv.Atoi(part); err != nil {
				continue
			}
		}
		switch {
		case code == 0:
			*st = sgrState{}
		case code == 1:
			st.bold = true
		case code == 2:
			st.dim = true
		case code == 3:
			st.italic = true
		case code == 4:
			st.underline = true
		case code == 7:
			st.reverse = true
		case code == 9:
			st.strikethrough = true
		case code == 22:
			st.bold, st.dim = false, false
		case code == 23:
			st.italic = false
		case code == 24:
			st.underline = false
		case code == 27:
			st.reverse = false
		case code == 29:
			st.strikethrough = false
		case code >= 30 && code <= 37:
			st.fg = p[code-30]
		case code == 39:
			st.fg = color.NRGBA{}
		case code >= 40 && code <= 47:
			st.bg = p[code-40]
		case code == 49:
			st.bg = color.NRGBA{}
		case code >= 90 && code <= 97:
			st.fg = p[code-90+8]
		case code >= 100 && code <= 107:
			st.bg = p[code-100+8]
		case code == 38, code == 48, code == 58:
			// Extended color: "5;n" or "2;r;g;b" in the following parameters.
			c, used := extColor(parts[i+1:], false, p)
			i += used
			switch code {
			case 38:
				st.fg = c
			case 48:
				st.bg = c
			} // 58 (underline color) is consumed but not supported
		}
	}
}

// applySub handles colon-separated parameters like "38:2::r:g:b" or "4:3".
func (st *sgrState) applySub(subs []string, p *Palette) {
	switch subs[0] {
	case "38", "48":
		if c, used := extColor(subs[1:], true, p); used > 0 {
			if subs[0] == "38" {
				st.fg = c
			} else {
				st.bg = c
			}
		}
	case "4":
		st.underline = len(subs) < 2 || (subs[1] != "0" && subs[1] != "")
	}
}

// extColor parses the arguments after 38/48/58 and returns the color and the
// number of arguments consumed. With colon syntax the truecolor form may carry
// a color space id before r;g;b.
func extColor(args []string, colon bool, p *Palette) (color.NRGBA, int) {
	if len(args) == 0 {
		return color.NRGBA{}, 0
	}
	switch args[0] {
	case "5":
		if len(args) < 2 {
			return color.NRGBA{}, len(args)
		}
		return color256(atoiClamp(args[1]), p), 2
	case "2":
		rgb := args[1:]
		if colon && len(rgb) >= 4 {
			rgb = rgb[1:]
		}
		if len(rgb) < 3 {
			return color.NRGBA{}, len(args)
		}
		c := color.NRGBA{uint8(atoiClamp(rgb[0])), uint8(atoiClamp(rgb[1])), uint8(atoiClamp(rgb[2])), 255}
		return c, len(args) - len(rgb) + 3
	}
	return color.NRGBA{}, 1
}

func atoiClamp(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return min(max(n, 0), 255)
}

// color256 maps an xterm 256-color index to a color.
func color256(n int, p *Palette) color.NRGBA {
	switch {
	case n < 16:
		return p[n]
	case n < 232:
		n -= 16
		level := func(v int) uint8 {
			if v == 0 {
				return 0
			}
			return uint8(55 + 40*v)
		}
		return color.NRGBA{level(n / 36), level(n / 6 % 6), level(n % 6), 255}
	default:
		g := uint8(8 + 10*(n-232))
		return color.NRGBA{g, g, g, 255}
	}
}
