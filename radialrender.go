package overlay

/*
#cgo pkg-config: pangocairo
#include <pango/pangocairo.h>
*/
import "C"

import (
	"image/color"
	"math"
	"runtime"
	"unsafe"
)

// renderRadial clears the w×h ARGB8888 buffer and draws v's ring into it
// (nothing for a nil v) at the given buffer scale. It returns the ring's
// area in buffer pixels, empty for nil.
func renderRadial(data unsafe.Pointer, w, h, stride, scale int, v *radialView) box {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	surf := C.cairo_image_surface_create_for_data((*C.uchar)(data), C.CAIRO_FORMAT_ARGB32,
		C.int(w), C.int(h), C.int(stride))
	defer C.cairo_surface_destroy(surf)
	cr := C.cairo_create(surf)
	defer C.cairo_destroy(cr)

	C.cairo_set_operator(cr, C.CAIRO_OPERATOR_SOURCE)
	C.cairo_set_source_rgba(cr, 0, 0, 0, 0)
	C.cairo_paint(cr)
	C.cairo_set_operator(cr, C.CAIRO_OPERATOR_OVER)
	if v == nil {
		C.cairo_surface_flush(surf)
		return box{}
	}

	s := float64(scale)
	C.cairo_scale(cr, C.double(s), C.double(s))
	st := v.style
	outer := st.Radius + ringBorder

	setColor(cr, st.Background)
	C.cairo_new_path(cr)
	C.cairo_arc(cr, C.double(v.x), C.double(v.y), C.double(outer), 0, 2*math.Pi)
	C.cairo_fill(cr)

	if n := len(v.items); n > 0 {
		step := 360 / float64(n)
		mid := (st.CenterRadius + st.Radius) / 2
		// A label may use the chord across its slot at mid radius, but no
		// more than the ring is wide (for rings of one or two items).
		maxW := math.Min(2*mid*math.Sin(math.Min(step, 180)/2*math.Pi/180), 2*(st.Radius-st.CenterRadius)) - 10
		for i, it := range v.items {
			if it.Label == "" {
				continue
			}
			fill := st.Slot
			switch {
			case it.Disabled:
				fill = st.Disabled
			case v.hover == (target{zone: zoneSlot, slot: i}):
				fill = st.Hover
			}
			a := float64(i) * step
			sector(cr, v.x, v.y, st.CenterRadius+ringInset, st.Radius, a-step/2+st.Gap/2, a+step/2-st.Gap/2)
			setColor(cr, fill)
			C.cairo_fill(cr)

			lx, ly := polar(v.x, v.y, mid, a)
			text := st.Text
			if it.Disabled {
				text = st.Muted
			}
			if it.Detail == "" {
				drawText(cr, []Span{{Text: it.Label}}, st.Font, text, lx, ly, maxW, true)
				continue
			}
			drawText(cr, []Span{{Text: it.Label}}, st.Font, text, lx, ly-7, maxW, true)
			drawText(cr, []Span{{Text: it.Detail}}, st.DetailFont, st.Muted, lx, ly+8, maxW, true)
		}
	}

	fill := st.Middle
	if v.middleActive && v.hover.zone == zoneCenter {
		fill = st.Hover
	}
	setColor(cr, fill)
	C.cairo_new_path(cr)
	C.cairo_arc(cr, C.double(v.x), C.double(v.y), C.double(st.CenterRadius), 0, 2*math.Pi)
	C.cairo_fill(cr)
	if len(v.middle) > 0 {
		drawText(cr, v.middle, st.CenterFont, st.Text, v.x, v.y, 2*st.CenterRadius-14, false)
	}
	C.cairo_surface_flush(surf)

	x0 := int(math.Floor((v.x - outer) * s))
	y0 := int(math.Floor((v.y - outer) * s))
	x1 := int(math.Ceil((v.x + outer) * s))
	y1 := int(math.Ceil((v.y + outer) * s))
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, w), min(y1, h)
	return box{x0, y0, x1 - x0, y1 - y0}
}

func setColor(cr *C.cairo_t, c color.NRGBA) {
	C.cairo_set_source_rgba(cr, C.double(c.R)/255, C.double(c.G)/255, C.double(c.B)/255, C.double(c.A)/255)
}

// polar returns the point r away from cx, cy in the direction deg degrees
// clockwise from the top.
func polar(cx, cy, r, deg float64) (float64, float64) {
	a := (deg - 90) * math.Pi / 180
	return cx + r*math.Cos(a), cy + r*math.Sin(a)
}

// sector makes the path of the ring sector between radii r0 and r1 and
// directions a0 to a1 (degrees clockwise from the top).
func sector(cr *C.cairo_t, cx, cy, r0, r1, a0, a1 float64) {
	t0, t1 := (a0-90)*math.Pi/180, (a1-90)*math.Pi/180
	C.cairo_new_path(cr)
	C.cairo_arc(cr, C.double(cx), C.double(cy), C.double(r1), C.double(t0), C.double(t1))
	C.cairo_arc_negative(cr, C.double(cx), C.double(cy), C.double(r0), C.double(t1), C.double(t0))
	C.cairo_close_path(cr)
}

// drawText draws spans centred on x, y, at most maxW wide: cut short with
// "…" if oneLine, else wrapped.
func drawText(cr *C.cairo_t, spans []Span, font string, fg color.NRGBA, x, y, maxW float64, oneLine bool) {
	l := Label{Spans: spans, Style: Style{Font: font, Foreground: fg, Align: AlignCenter}}
	layout := newLayout(cr, &l, TopLeft)
	defer C.g_object_unref(C.gpointer(layout))
	maxW = math.Max(maxW, 1)
	C.pango_layout_set_width(layout, C.int(maxW*C.PANGO_SCALE))
	if oneLine {
		C.pango_layout_set_ellipsize(layout, C.PANGO_ELLIPSIZE_END)
	} else {
		C.pango_layout_set_wrap(layout, C.PANGO_WRAP_WORD_CHAR)
	}
	var lw, lh C.int
	C.pango_layout_get_pixel_size(layout, &lw, &lh)
	C.cairo_move_to(cr, C.double(x-maxW/2), C.double(y-float64(lh)/2))
	C.pango_cairo_show_layout(cr, layout)
}
