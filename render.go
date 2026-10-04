package overlay

/*
#cgo pkg-config: pangocairo
#include <stdlib.h>
#include <pango/pangocairo.h>
*/
import "C"

import (
	"math"
	"runtime"
	"strings"
	"unsafe"
)

// The default Pango font map is per thread, so every function here pins the
// goroutine to its thread for the lifetime of the Pango objects it creates.

// newLayout builds a Pango layout for l. The caller must g_object_unref it.
func newLayout(cr *C.cairo_t, l *Label, c Corner) *C.PangoLayout {
	layout := C.pango_cairo_create_layout(cr)

	// Metrics hinting off keeps the size independent of the buffer scale;
	// grayscale AA because subpixel AA looks wrong on a transparent surface.
	fo := C.cairo_font_options_create()
	C.cairo_font_options_set_hint_metrics(fo, C.CAIRO_HINT_METRICS_OFF)
	C.cairo_font_options_set_antialias(fo, C.CAIRO_ANTIALIAS_GRAY)
	C.pango_cairo_context_set_font_options(C.pango_layout_get_context(layout), fo)
	C.cairo_font_options_destroy(fo)
	C.pango_layout_context_changed(layout)

	cfont := C.CString(l.Style.Font)
	desc := C.pango_font_description_from_string(cfont)
	C.free(unsafe.Pointer(cfont))
	C.pango_layout_set_font_description(layout, desc)
	C.pango_font_description_free(desc)

	var text strings.Builder
	attrs := C.pango_attr_list_new()
	for _, sp := range l.Spans {
		t := strings.ToValidUTF8(sp.Text, "�")
		if t == "" {
			continue
		}
		start := C.guint(text.Len())
		text.WriteString(t)
		end := C.guint(text.Len())

		col := sp.Color
		if col.A == 0 {
			col = l.Style.Foreground
		}
		insert := func(a *C.PangoAttribute) {
			a.start_index, a.end_index = start, end
			C.pango_attr_list_insert(attrs, a)
		}
		insert(C.pango_attr_foreground_new(C.guint16(col.R)*257, C.guint16(col.G)*257, C.guint16(col.B)*257))
		insert(C.pango_attr_foreground_alpha_new(C.guint16(col.A) * 257))
		if sp.Bold {
			insert(C.pango_attr_weight_new(C.PANGO_WEIGHT_BOLD))
		}
		if sp.Italic {
			insert(C.pango_attr_style_new(C.PANGO_STYLE_ITALIC))
		}
	}
	ctext := C.CString(text.String())
	C.pango_layout_set_text(layout, ctext, -1)
	C.free(unsafe.Pointer(ctext))
	C.pango_layout_set_attributes(layout, attrs)
	C.pango_attr_list_unref(attrs)

	align := C.PangoAlignment(C.PANGO_ALIGN_LEFT)
	switch {
	case l.Style.Align == AlignCenter:
		align = C.PANGO_ALIGN_CENTER
	case l.Style.Align == AlignRight, l.Style.Align == AlignAuto && !c.left():
		align = C.PANGO_ALIGN_RIGHT
	}
	C.pango_layout_set_alignment(layout, align)
	return layout
}

// measure returns the size of the rendered label in logical pixels.
func measure(l *Label) (w, h int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	surf := C.cairo_image_surface_create(C.CAIRO_FORMAT_ARGB32, 1, 1)
	defer C.cairo_surface_destroy(surf)
	cr := C.cairo_create(surf)
	defer C.cairo_destroy(cr)
	layout := newLayout(cr, l, TopLeft)
	defer C.g_object_unref(C.gpointer(layout))

	var cw, ch C.int
	C.pango_layout_get_pixel_size(layout, &cw, &ch)
	pad := max(l.Style.Padding, 0)
	// Layer surfaces anchored to two adjacent edges must not be 0x0.
	return max(int(cw)+2*pad, 1), max(int(ch)+2*pad, 1)
}

// render draws l into an ARGB8888 (premultiplied, native endian) buffer of
// w×h pixels at the given buffer scale. data must stay valid for the call only.
func render(l *Label, c Corner, data unsafe.Pointer, w, h, stride, scale int) {
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

	s := float64(scale)
	C.cairo_scale(cr, C.double(s), C.double(s))
	lw, lh := float64(w)/s, float64(h)/s

	if bg := l.Style.Background; bg.A > 0 {
		roundedRect(cr, lw, lh, min(l.Style.Radius, lw/2, lh/2))
		C.cairo_set_source_rgba(cr, C.double(bg.R)/255, C.double(bg.G)/255, C.double(bg.B)/255, C.double(bg.A)/255)
		C.cairo_fill(cr)
	}

	layout := newLayout(cr, l, c)
	defer C.g_object_unref(C.gpointer(layout))
	pad := float64(max(l.Style.Padding, 0))
	C.cairo_move_to(cr, C.double(pad), C.double(pad))
	C.pango_cairo_show_layout(cr, layout)
	C.cairo_surface_flush(surf)
}

func roundedRect(cr *C.cairo_t, w, h, r float64) {
	if r <= 0 {
		C.cairo_rectangle(cr, 0, 0, C.double(w), C.double(h))
		return
	}
	arc := func(x, y, a1, a2 float64) {
		C.cairo_arc(cr, C.double(x), C.double(y), C.double(r), C.double(a1), C.double(a2))
	}
	C.cairo_new_sub_path(cr)
	arc(w-r, r, -math.Pi/2, 0)
	arc(w-r, h-r, 0, math.Pi/2)
	arc(r, h-r, math.Pi/2, math.Pi)
	arc(r, r, math.Pi, 3*math.Pi/2)
	C.cairo_close_path(cr)
}
