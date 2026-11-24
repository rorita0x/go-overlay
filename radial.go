package overlay

import (
	"image/color"
	"math"
	"time"
)

// RadialItem is one slot of a radial menu ring.
type RadialItem struct {
	// Label is drawn in the slot. An empty Label leaves the slot blank: not
	// drawn and not clickable, so the other items keep their directions.
	Label string
	// Detail is an optional, smaller second line, e.g. "3 entries ›".
	Detail string
	// Value comes back in RadialPick; empty means Label.
	Value string
	// Disabled draws the slot dimmed; clicking it does nothing.
	Disabled bool
	// Preview returns what the middle shows while the slot is hovered; nil
	// shows Label. It runs on Run's thread at every redraw and must be fast.
	Preview func() []Span
	// Children make the slot open a submenu: a new ring centred where the
	// slot was clicked.
	Children []RadialItem
}

// Radial is a radial (pie) menu, opened where the mouse pointer is. See
// SetRadial.
type Radial struct {
	// Items fill the ring clockwise from the top, each taking 360°/len(Items).
	Items []RadialItem
	// Center is the middle of the top ring: its Preview (or Label) is shown
	// there while nothing else is hovered, and clicking it picks it
	// (RadialPick.Center) unless it is Disabled or has neither Value nor
	// Label. In a submenu the middle goes back instead.
	Center RadialItem
	// Back is what a submenu's middle says below the submenu's name; default
	// "‹ back".
	Back  string
	Style RadialStyle
	// Refresh redraws the menu this often while it is open, for live
	// previews such as a countdown; 0 redraws only on changes.
	Refresh time.Duration
	// OnPick is called after the menu has closed because an item (or the top
	// ring's middle) was clicked. It runs on Run's thread and must not block.
	OnPick func(RadialPick)
	// OnCancel is called after the menu has closed because a click landed
	// outside the ring. It runs on Run's thread and must not block.
	OnCancel func()
}

// RadialPick is the item picked from a radial menu.
type RadialPick struct {
	Path   []int  // indices into Items, then into each submenu's Children
	Center bool   // the top ring's middle was picked; Path is empty
	Value  string // the item's Value, or its Label if that is empty
	Button Button // the mouse button that was released on it
}

// RadialStyle controls the look of a radial menu. Zero fields take the
// value from DefaultRadialStyle.
type RadialStyle struct {
	Radius       float64 // outer radius of the slots, in logical pixels
	CenterRadius float64 // radius of the middle; default 0.38 × Radius
	Gap          float64 // degrees left free between two slots
	Margin       float64 // distance a ring keeps from the screen edges
	Font         string  // slot labels
	DetailFont   string  // a slot's second line
	CenterFont   string  // the middle
	Background   color.NRGBA
	Slot         color.NRGBA
	Hover        color.NRGBA
	Disabled     color.NRGBA // fill of a disabled slot
	Middle       color.NRGBA // fill of the middle
	Text         color.NRGBA
	Muted        color.NRGBA // details and disabled labels
	Accent       color.NRGBA // the "back" line in a submenu's middle
}

// DefaultRadialStyle matches the colors of DefaultTextStyle.
var DefaultRadialStyle = RadialStyle{
	Radius:     120,
	Gap:        2.5,
	Margin:     8,
	Font:       "Sans Bold 10",
	DetailFont: "Sans 8",
	CenterFont: "Sans 10",
	Background: MustHex("#1e1e2ef2"),
	Slot:       MustHex("#313244f5"),
	Hover:      MustHex("#45475a"),
	Disabled:   MustHex("#3132448c"),
	Middle:     MustHex("#181825"),
	Text:       MustHex("#cdd6f4"),
	Muted:      MustHex("#7f849c"),
	Accent:     MustHex("#89b4fa"),
}

func (s RadialStyle) withDefaults() RadialStyle {
	d := DefaultRadialStyle
	num := func(v *float64, def float64) {
		if *v <= 0 {
			*v = def
		}
	}
	str := func(v *string, def string) {
		if *v == "" {
			*v = def
		}
	}
	col := func(v *color.NRGBA, def color.NRGBA) {
		if *v == (color.NRGBA{}) {
			*v = def
		}
	}
	num(&s.Radius, d.Radius)
	num(&s.CenterRadius, 0.38*s.Radius)
	num(&s.Gap, d.Gap)
	num(&s.Margin, d.Margin)
	str(&s.Font, d.Font)
	str(&s.DetailFont, d.DetailFont)
	str(&s.CenterFont, d.CenterFont)
	col(&s.Background, d.Background)
	col(&s.Slot, d.Slot)
	col(&s.Hover, d.Hover)
	col(&s.Disabled, d.Disabled)
	col(&s.Middle, d.Middle)
	col(&s.Text, d.Text)
	col(&s.Muted, d.Muted)
	col(&s.Accent, d.Accent)
	return s
}

// SetRadial opens r where the mouse pointer is, replacing a menu that is
// open. The menu takes pointer input on the whole screen while it is open
// (a click outside the ring cancels it) but never the keyboard focus, so
// the focused window keeps it. Safe to call from any goroutine.
func (o *Overlay) SetRadial(r Radial) {
	r.Style = r.Style.withDefaults()
	if r.Back == "" {
		r.Back = "‹ back"
	}
	r.Items = cloneItems(r.Items)
	r.Center.Children = nil
	o.mu.Lock()
	o.radialReq = &r
	o.radialGen++
	o.mu.Unlock()
	o.radialShown.Store(true)
	o.wake()
}

// ClearRadial closes the radial menu without calling OnPick or OnCancel.
// Safe to call from any goroutine.
func (o *Overlay) ClearRadial() {
	o.mu.Lock()
	o.radialReq = nil
	o.radialGen++
	o.mu.Unlock()
	o.radialShown.Store(false)
	o.wake()
}

// RadialOpen reports whether a radial menu is open, e.g. to make one
// shortcut both open and close it.
func (o *Overlay) RadialOpen() bool {
	return o.radialShown.Load()
}

func cloneItems(items []RadialItem) []RadialItem {
	if items == nil {
		return nil
	}
	out := make([]RadialItem, len(items))
	for i, it := range items {
		it.Children = cloneItems(it.Children)
		out[i] = it
	}
	return out
}

// ringBorder is the width of the background around the slots, and
// ringInset the gap between the middle and the slots.
const (
	ringBorder = 4
	ringInset  = 3
)

// zone is where a point lies relative to a ring.
type zone int

const (
	zoneOutside zone = iota // beyond the ring: a click there cancels
	zoneCenter              // the middle
	zoneSlot                // a slot with an item
	zoneBlank               // inside the ring, but on a gap, a blank slot or the border
)

type target struct {
	zone zone
	slot int
}

// hitTest returns what lies at x, y for a ring of items centred at cx, cy.
func hitTest(items []RadialItem, cx, cy, x, y float64, st RadialStyle) target {
	dx, dy := x-cx, y-cy
	d := math.Hypot(dx, dy)
	switch {
	case d <= st.CenterRadius:
		return target{zone: zoneCenter}
	case d > st.Radius+ringBorder:
		return target{zone: zoneOutside}
	case d > st.Radius || d < st.CenterRadius+ringInset || len(items) == 0:
		return target{zone: zoneBlank}
	}
	// Degrees clockwise from the top (screen y grows downwards).
	a := math.Mod(math.Atan2(dx, -dy)*180/math.Pi+360, 360)
	step := 360 / float64(len(items))
	slot := int(math.Floor((a+step/2)/step)) % len(items)
	off := math.Mod(a-float64(slot)*step+540, 360) - 180
	if math.Abs(off) > step/2-st.Gap/2 || items[slot].Label == "" {
		return target{zone: zoneBlank}
	}
	return target{zone: zoneSlot, slot: slot}
}

// clampCenter moves a ring's middle so the whole ring, extent from its
// middle, stays inside a w×h output; on an output too small for it, the
// ring is centred.
func clampCenter(x, y, w, h, extent float64) (float64, float64) {
	clamp := func(v, size float64) float64 {
		if size < 2*extent {
			return size / 2
		}
		return math.Min(math.Max(v, extent), size-extent)
	}
	return clamp(x, w), clamp(y, h)
}

// ring is one level of an open radial menu.
type ring struct {
	items []RadialItem
	x, y  float64 // middle, in the owning surface's logical pixels
	slot  int     // the parent's slot that opened it
	label string  // that slot's label
}

// radialMenu is the state of an open radial menu: which surface shows it,
// its rings (the last one is shown), hover and press. It is independent of
// Wayland; redraw and close are bound by the overlay.
type radialMenu struct {
	r     Radial
	rings []ring
	owner *radialSurface // the surface the rings are on; nil until the pointer entered one
	w, h  float64        // owner's logical size

	hover    target
	press    target
	pressBtn Button
	pressed  bool
	px, py   float64
	done     bool

	redraw func()
	close  func()
}

func newRadialMenu(r Radial) *radialMenu {
	return &radialMenu{r: r, redraw: func() {}, close: func() {}}
}

func (m *radialMenu) extent() float64 {
	return m.r.Style.Radius + ringBorder + m.r.Style.Margin
}

func (m *radialMenu) cur() ring { return m.rings[len(m.rings)-1] }

// targetAt returns what lies under the pointer at x, y on surface s.
func (m *radialMenu) targetAt(s *radialSurface, x, y float64) target {
	if m.owner == nil || s != m.owner {
		return target{zone: zoneOutside}
	}
	c := m.cur()
	return hitTest(c.items, c.x, c.y, x, y, m.r.Style)
}

// enter places the top ring at the pointer the first time it enters one of
// the menu's surfaces (of w×h logical pixels).
func (m *radialMenu) enter(s *radialSurface, x, y, w, h float64) {
	if m.done {
		return
	}
	if m.owner == nil {
		m.owner, m.w, m.h = s, w, h
		cx, cy := clampCenter(x, y, w, h, m.extent())
		m.rings = []ring{{items: m.r.Items, x: cx, y: cy}}
		m.hover = target{zone: -1} // force the first draw
	}
	m.motion(s, x, y)
}

func (m *radialMenu) motion(s *radialSurface, x, y float64) {
	if m.done {
		return
	}
	m.px, m.py = x, y
	if t := m.targetAt(s, x, y); t != m.hover {
		m.hover = t
		m.redraw()
	}
}

func (m *radialMenu) leave(s *radialSurface) {
	if m.done || s != m.owner {
		return
	}
	m.pressed = false
	if m.hover != (target{zone: zoneOutside}) {
		m.hover = target{zone: zoneOutside}
		m.redraw()
	}
}

// button handles a press or release at the last pointer position on s. A
// release acts only on what the same button was pressed on.
func (m *radialMenu) button(s *radialSurface, b Button, down bool) {
	if m.done {
		return
	}
	t := m.targetAt(s, m.px, m.py)
	if down {
		m.press, m.pressBtn, m.pressed = t, b, true
		return
	}
	if !m.pressed || b != m.pressBtn || t != m.press {
		m.pressed = false
		return
	}
	m.pressed = false
	m.act(t, b)
}

func (m *radialMenu) act(t target, b Button) {
	switch t.zone {
	case zoneOutside:
		m.finish()
		if m.r.OnCancel != nil {
			m.r.OnCancel()
		}
	case zoneCenter:
		if len(m.rings) > 1 {
			m.rings = m.rings[:len(m.rings)-1]
			m.hover = m.targetAt(m.owner, m.px, m.py)
			m.redraw()
			return
		}
		c := m.r.Center
		if c.Disabled || c.Value == "" && c.Label == "" {
			return
		}
		m.pick(RadialPick{Center: true, Value: itemValue(c), Button: b})
	case zoneSlot:
		it := m.cur().items[t.slot]
		if it.Disabled {
			return
		}
		if it.Children != nil {
			cx, cy := clampCenter(m.px, m.py, m.w, m.h, m.extent())
			m.rings = append(m.rings, ring{items: it.Children, x: cx, y: cy, slot: t.slot, label: it.Label})
			m.hover = m.targetAt(m.owner, m.px, m.py)
			m.redraw()
			return
		}
		path := make([]int, 0, len(m.rings))
		for _, r := range m.rings[1:] {
			path = append(path, r.slot)
		}
		m.pick(RadialPick{Path: append(path, t.slot), Value: itemValue(it), Button: b})
	}
}

func itemValue(it RadialItem) string {
	if it.Value != "" {
		return it.Value
	}
	return it.Label
}

func (m *radialMenu) pick(p RadialPick) {
	m.finish()
	if m.r.OnPick != nil {
		m.r.OnPick(p)
	}
}

func (m *radialMenu) finish() {
	m.done = true
	m.close()
}

// radialView is what one frame of the menu shows.
type radialView struct {
	x, y         float64
	items        []RadialItem
	hover        target
	middle       []Span
	middleActive bool // the middle does something when clicked
	style        RadialStyle
}

// view returns the frame to draw, ok=false while the menu isn't placed.
func (m *radialMenu) view() (v radialView, ok bool) {
	if m.owner == nil || len(m.rings) == 0 {
		return v, false
	}
	c := m.cur()
	v = radialView{x: c.x, y: c.y, items: c.items, hover: m.hover, style: m.r.Style}
	sub := len(m.rings) > 1
	v.middleActive = sub || !m.r.Center.Disabled && (m.r.Center.Value != "" || m.r.Center.Label != "")
	switch {
	case m.hover.zone == zoneSlot:
		it := c.items[m.hover.slot]
		v.middle = previewOf(it)
	case sub:
		v.middle = []Span{{Text: c.label + "\n", Bold: true}, {Text: m.r.Back, Color: m.r.Style.Accent}}
	default:
		v.middle = previewOf(m.r.Center)
	}
	return v, true
}

func previewOf(it RadialItem) []Span {
	if it.Preview != nil {
		return it.Preview()
	}
	if it.Label == "" {
		return nil
	}
	return []Span{{Text: it.Label, Bold: true}}
}
