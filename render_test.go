package overlay

import (
	"image/color"
	"testing"
	"unsafe"
)

func label(style Style, spans ...Span) Label {
	return Label{Spans: spans, Style: style}.clone()
}

func TestMeasure(t *testing.T) {
	base := Style{Font: "Sans 11"}
	short := label(base, Span{Text: "ab"})
	sw, sh := measure(&short)

	tests := []struct {
		name          string
		l             Label
		wider, taller bool
	}{
		{"longer text", label(base, Span{Text: "abcdefgh"}), true, false},
		{"two lines", label(base, Span{Text: "ab\ncd"}), false, true},
		{"split spans", label(base, Span{Text: "abcd"}, Span{Text: "efgh", Bold: true}), true, false},
		{"padding", label(Style{Font: "Sans 11", Padding: 10}, Span{Text: "ab"}), true, true},
		{"bigger font", label(Style{Font: "Sans 22"}, Span{Text: "ab"}), true, true},
	}
	for _, tt := range tests {
		w, h := measure(&tt.l)
		if tt.wider != (w > sw) || tt.taller != (h > sh) {
			t.Errorf("%s: size %dx%d vs base %dx%d, want wider=%v taller=%v", tt.name, w, h, sw, sh, tt.wider, tt.taller)
		}
	}

	empty := label(base)
	if w, h := measure(&empty); w < 1 || h < 1 {
		t.Errorf("empty label size %dx%d, want at least 1x1", w, h)
	}
}

// pixel returns the unpremultiplied-agnostic raw ARGB components at x,y.
func pixel(buf []byte, stride, x, y int) (a, r, g, b uint8) {
	p := buf[y*stride+x*4:]
	return p[3], p[2], p[1], p[0] // little endian ARGB8888
}

func renderBuf(t *testing.T, l Label, scale int) ([]byte, int, int, int) {
	t.Helper()
	w, h := measure(&l)
	w, h = w*scale, h*scale
	stride := w * 4
	buf := make([]byte, stride*h)
	render(&l, TopLeft, unsafe.Pointer(&buf[0]), w, h, stride, scale)
	return buf, w, h, stride
}

func TestRenderColors(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}

	tests := []struct {
		name  string
		l     Label
		scale int
		check func(t *testing.T, buf []byte, w, h, stride int)
	}{
		{
			name:  "background in padding",
			l:     label(Style{Padding: 6, Background: blue}, Span{Text: "x"}),
			scale: 1,
			check: func(t *testing.T, buf []byte, w, h, stride int) {
				if a, r, g, b := pixel(buf, stride, 3, h/2); a != 255 || r != 0 || g != 0 || b != 255 {
					t.Errorf("padding pixel = %d,%d,%d,%d, want opaque blue", a, r, g, b)
				}
			},
		},
		{
			name:  "transparent without background",
			l:     label(Style{Padding: 6}, Span{Text: "x"}),
			scale: 2,
			check: func(t *testing.T, buf []byte, w, h, stride int) {
				if a, _, _, _ := pixel(buf, stride, 2, 2); a != 0 {
					t.Errorf("corner alpha = %d, want 0", a)
				}
			},
		},
		{
			name:  "span colors",
			l:     label(Style{}, Span{Text: "████", Color: red}, Span{Text: "████", Color: blue}),
			scale: 1,
			check: func(t *testing.T, buf []byte, w, h, stride int) {
				// Left quarter must be red only, right quarter blue only.
				var leftR, rightB int
				for y := range h {
					for x := range w {
						_, r, g, b := pixel(buf, stride, x, y)
						switch {
						case x < w/4:
							if g != 0 || b != 0 {
								t.Fatalf("left pixel %d,%d has g=%d b=%d", x, y, g, b)
							}
							leftR += int(r)
						case x >= 3*w/4:
							if r != 0 || g != 0 {
								t.Fatalf("right pixel %d,%d has r=%d g=%d", x, y, r, g)
							}
							rightB += int(b)
						}
					}
				}
				if leftR == 0 || rightB == 0 {
					t.Errorf("no text drawn: red sum %d, blue sum %d", leftR, rightB)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, w, h, stride := renderBuf(t, tt.l, tt.scale)
			tt.check(t, buf, w, h, stride)
		})
	}
}
