package overlay

import (
	"image/color"
	"reflect"
	"testing"
)

func TestParseANSI(t *testing.T) {
	p := &DefaultPalette
	red, green, brightBlue := p[1], p[2], p[12]
	rgb := func(r, g, b uint8) color.NRGBA { return color.NRGBA{r, g, b, 255} }

	tests := []struct {
		name string
		in   string
		want []Span
	}{
		{"plain", "hello", []Span{{Text: "hello"}}},
		{"empty", "", nil},
		{"only escapes", "\x1b[31m\x1b[0m", nil},
		{"trailing newline", "a\nb\n", []Span{{Text: "a\nb"}}},
		{"trailing newline after reset", "\x1b[32mok\n\x1b[0m\n", []Span{{Text: "ok", Color: green}}},
		{"fg and reset", "\x1b[31mred\x1b[0m plain", []Span{{Text: "red", Color: red}, {Text: " plain"}}},
		{"empty params reset", "\x1b[31mr\x1b[mx", []Span{{Text: "r", Color: red}, {Text: "x"}}},
		{"bright fg", "\x1b[94mb", []Span{{Text: "b", Color: brightBlue}}},
		{"bg", "\x1b[42mg\x1b[49mx", []Span{{Text: "g", Background: green}, {Text: "x"}}},
		{"bright bg", "\x1b[101mx", []Span{{Text: "x", Background: p[9]}}},
		{"combined params", "\x1b[1;3;4;9;31mx", []Span{{Text: "x", Color: red, Bold: true, Italic: true, Underline: true, Strikethrough: true}}},
		{"attribute offs", "\x1b[1;3;4;9mx\x1b[22;23;24;29my", []Span{{Text: "x", Bold: true, Italic: true, Underline: true, Strikethrough: true}, {Text: "y"}}},
		{"default fg", "\x1b[31ma\x1b[39mb", []Span{{Text: "a", Color: red}, {Text: "b"}}},
		{"256 palette", "\x1b[38;5;9mx", []Span{{Text: "x", Color: p[9]}}},
		{"256 cube", "\x1b[38;5;196mx", []Span{{Text: "x", Color: rgb(255, 0, 0)}}},
		{"256 cube mixed", "\x1b[38;5;110mx", []Span{{Text: "x", Color: rgb(135, 175, 215)}}},
		{"256 grey", "\x1b[48;5;232mx", []Span{{Text: "x", Background: rgb(8, 8, 8)}}},
		{"truecolor", "\x1b[38;2;10;20;30mx", []Span{{Text: "x", Color: rgb(10, 20, 30)}}},
		{"truecolor then attr", "\x1b[38;2;10;20;30;1mx", []Span{{Text: "x", Color: rgb(10, 20, 30), Bold: true}}},
		{"truecolor colon", "\x1b[38:2:10:20:30mx", []Span{{Text: "x", Color: rgb(10, 20, 30)}}},
		{"truecolor colon colorspace", "\x1b[48:2::10:20:30mx", []Span{{Text: "x", Background: rgb(10, 20, 30)}}},
		{"truecolor clamped", "\x1b[38;2;300;0;999mx", []Span{{Text: "x", Color: rgb(255, 0, 255)}}},
		{"underline color ignored", "\x1b[58;2;1;2;3;4mx", []Span{{Text: "x", Underline: true}}},
		{"curly underline", "\x1b[4:3mx\x1b[4:0my", []Span{{Text: "x", Underline: true}, {Text: "y"}}},
		{"reverse default", "\x1b[7mx", []Span{{Text: "x", Color: black, Background: white}}},
		{"reverse colored", "\x1b[31;7mx", []Span{{Text: "x", Color: black, Background: red}}},
		{"dim", "\x1b[2;31mx", []Span{{Text: "x", Color: color.NRGBA{red.R, red.G, red.B, 127}}}},
		{"dim default", "\x1b[2mx", []Span{{Text: "x", Color: color.NRGBA{255, 255, 255, 127}}}},
		{"cursor and erase stripped", "\x1b[2J\x1b[Ha\x1b[Kb\x1b[?25l", []Span{{Text: "ab"}}},
		{"private sgr ignored", "\x1b[>4;2mx", []Span{{Text: "x"}}},
		{"osc hyperlink", "\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\", []Span{{Text: "link"}}},
		{"osc title bel", "\x1b]0;title\x07x", []Span{{Text: "x"}}},
		{"charset designation", "\x1b(Bx", []Span{{Text: "x"}}},
		{"controls", "a\r\x07\bb\tc", []Span{{Text: "ab\tc"}}},
		{"c1 control", "a\u009bb", []Span{{Text: "ab"}}},
		{"utf8 kept", "\x1b[31mgrün ✓", []Span{{Text: "grün ✓", Color: red}}},
		{"malformed csi swallowed", "\x1b[38;2;-5;0mx", []Span{{Text: "x"}}},
		{"truncated csi", "x\x1b[31", []Span{{Text: "x"}}},
		{"truncated 256", "\x1b[38;5mx", []Span{{Text: "x"}}},
		{"lone esc", "x\x1b", []Span{{Text: "x"}}},
		{"unterminated osc", "x\x1b]0;foo", []Span{{Text: "x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseANSI(tt.in, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseANSI(%q)\n got  %+v\n want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseANSIPalette(t *testing.T) {
	var p Palette
	p[1] = color.NRGBA{1, 2, 3, 255}
	got := ParseANSI("\x1b[31mx", &p)
	if len(got) != 1 || got[0].Color != p[1] {
		t.Errorf("custom palette not used: %+v", got)
	}
}
