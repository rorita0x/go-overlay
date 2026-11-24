package overlay

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestHitTest(t *testing.T) {
	st := RadialStyle{}.withDefaults() // radius 120, middle 45.6, gap 2.5°
	eight := []RadialItem{{Label: "N"}, {Label: "NE"}, {Label: "E"}, {Label: "SE"}, {Label: "S"}, {Label: ""}, {Label: "W"}, {Label: "NW"}}
	const cx, cy = 200, 200
	tests := []struct {
		name string
		x, y float64
		want target
	}{
		{"middle", 210, 195, target{zone: zoneCenter}},
		{"top", 200, 120, target{zone: zoneSlot, slot: 0}},
		{"right", 280, 200, target{zone: zoneSlot, slot: 2}},
		{"bottom", 200, 290, target{zone: zoneSlot, slot: 4}},
		{"left", 110, 200, target{zone: zoneSlot, slot: 6}},
		{"just left of top", 190, 110, target{zone: zoneSlot, slot: 0}},
		{"upper left", 140, 140, target{zone: zoneSlot, slot: 7}},
		{"blank slot", 140, 260, target{zone: zoneBlank}},
		{"on the gap between N and NE", 200 + 80*0.3827, 200 - 80*0.9239, target{zone: zoneBlank}}, // 22.5°
		{"between middle and slots", 200, 152, target{zone: zoneBlank}},
		{"on the border", 200, 77, target{zone: zoneBlank}},
		{"outside", 200, 60, target{zone: zoneOutside}},
		{"far away", 900, 900, target{zone: zoneOutside}},
	}
	for _, tt := range tests {
		if got := hitTest(eight, cx, cy, tt.x, tt.y, st); got != tt.want {
			t.Errorf("%s (%v,%v): got %+v, want %+v", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
	if got := hitTest(nil, cx, cy, 200, 120, st); got != (target{zone: zoneBlank}) {
		t.Errorf("a ring without items: %+v", got)
	}
}

func TestClampCenter(t *testing.T) {
	tests := []struct {
		name         string
		x, y, w, h   float64
		wantX, wantY float64
	}{
		{"fits", 500, 400, 1920, 1080, 500, 400},
		{"top left corner", 10, 5, 1920, 1080, 132, 132},
		{"bottom right corner", 1919, 1079, 1920, 1080, 1788, 948},
		{"output narrower than the ring", 50, 500, 200, 1080, 100, 500},
	}
	for _, tt := range tests {
		x, y := clampCenter(tt.x, tt.y, tt.w, tt.h, 132)
		if x != tt.wantX || y != tt.wantY {
			t.Errorf("%s: got %v,%v, want %v,%v", tt.name, x, y, tt.wantX, tt.wantY)
		}
	}
}

// testMenu is an open menu on a fake surface, recording picks, cancels and
// whether it was closed.
type testMenu struct {
	m       *radialMenu
	s       *radialSurface
	picks   []RadialPick
	cancels int
	closed  bool
}

func newTestMenu() *testMenu {
	tm := &testMenu{s: &radialSurface{}}
	r := Radial{
		Items: []RadialItem{
			{Label: "Leaf", Value: "leaf"},
			{Label: "Sub", Children: []RadialItem{
				{Label: "A"},
				{Label: "Deeper", Children: []RadialItem{{Label: "X", Value: "x"}, {Label: "Y"}}},
				{Label: "Off", Disabled: true},
			}},
			{Label: "Locked", Disabled: true},
			{Label: ""},
		},
		Center:   RadialItem{Label: "Login", Value: "login"},
		OnPick:   func(p RadialPick) { tm.picks = append(tm.picks, p) },
		OnCancel: func() { tm.cancels++ },
	}
	r.Style = r.Style.withDefaults()
	r.Back = "‹ back"
	tm.m = newRadialMenu(r)
	tm.m.close = func() { tm.closed = true }
	return tm
}

// click presses and releases b at x, y on the fake surface.
func (tm *testMenu) click(x, y float64, b Button) {
	tm.m.motion(tm.s, x, y)
	tm.m.button(tm.s, b, true)
	tm.m.button(tm.s, b, false)
}

func TestRadialMenuPicks(t *testing.T) {
	tm := newTestMenu()
	tm.m.enter(tm.s, 600, 500, 1920, 1080)
	if c := tm.m.cur(); c.x != 600 || c.y != 500 {
		t.Fatalf("top ring at %v,%v, want the pointer's 600,500", c.x, c.y)
	}

	tm.click(680, 500, ButtonLeft) // "Locked" (east, slot 2): nothing
	tm.click(600, 580, ButtonLeft) // blank (south): nothing
	if tm.closed || len(tm.picks) > 0 {
		t.Fatal("a disabled or blank slot did something")
	}

	tm.click(680, 520, ButtonRight) // slightly below east is still "Locked"
	tm.click(600, 420, ButtonRight) // "Leaf" (north)
	want := []RadialPick{{Path: []int{0}, Value: "leaf", Button: ButtonRight}}
	if !reflect.DeepEqual(tm.picks, want) || !tm.closed {
		t.Fatalf("picks = %+v (closed %v), want %+v", tm.picks, tm.closed, want)
	}
	tm.click(600, 420, ButtonLeft) // after closing, nothing more happens
	if len(tm.picks) != 1 {
		t.Fatal("a closed menu picked again")
	}
}

func TestRadialMenuSubmenusAndBack(t *testing.T) {
	tm := newTestMenu()
	tm.m.enter(tm.s, 600, 500, 1920, 1080)

	// "Sub" is east (slot 1 of 4): its ring opens where it was clicked.
	tm.click(680, 500, ButtonLeft)
	if n := len(tm.m.rings); n != 2 {
		t.Fatalf("%d rings, want the submenu open", n)
	}
	if c := tm.m.cur(); c.x != 680 || c.y != 500 {
		t.Fatalf("submenu at %v,%v, want the click point 680,500", c.x, c.y)
	}
	if v, _ := tm.m.view(); len(v.middle) != 2 || v.middle[1].Text != "‹ back" {
		t.Fatalf("submenu middle = %+v, want its name and back", v.middle)
	}

	// The middle goes back to the top ring, at its old place.
	tm.click(680, 500, ButtonLeft)
	if c := tm.m.cur(); len(tm.m.rings) != 1 || c.x != 600 || c.y != 500 {
		t.Fatalf("after back: %d rings, middle at %v,%v", len(tm.m.rings), c.x, c.y)
	}

	// Sub (east) → Deeper (south-east of a 3-slot ring is slot 1 at 120°)
	// → Y (slot 1 of 2: south).
	tm.click(680, 500, ButtonLeft)
	tm.click(680+69, 500+40, ButtonLeft)
	c := tm.m.cur()
	tm.click(c.x, c.y+80, ButtonMiddle)
	want := []RadialPick{{Path: []int{1, 1, 1}, Value: "Y", Button: ButtonMiddle}}
	if !reflect.DeepEqual(tm.picks, want) {
		t.Fatalf("picks = %+v, want %+v", tm.picks, want)
	}
}

func TestRadialMenuCenterCancelAndDrag(t *testing.T) {
	tm := newTestMenu()
	tm.m.enter(tm.s, 600, 500, 1920, 1080)

	// Pressed on a slot, released elsewhere: nothing.
	tm.m.motion(tm.s, 600, 420)
	tm.m.button(tm.s, ButtonLeft, true)
	tm.m.motion(tm.s, 600, 900)
	tm.m.button(tm.s, ButtonLeft, false)
	if tm.closed {
		t.Fatal("a drag off the slot closed the menu")
	}

	tm.click(600, 500, ButtonLeft) // the top ring's middle: Login
	want := []RadialPick{{Center: true, Value: "login", Button: ButtonLeft}}
	if !reflect.DeepEqual(tm.picks, want) {
		t.Fatalf("picks = %+v, want %+v", tm.picks, want)
	}

	tm = newTestMenu()
	tm.m.enter(tm.s, 600, 500, 1920, 1080)
	tm.click(1500, 900, ButtonLeft) // outside the ring
	if tm.cancels != 1 || !tm.closed || len(tm.picks) != 0 {
		t.Fatalf("outside click: cancels %d, closed %v, picks %v", tm.cancels, tm.closed, tm.picks)
	}

	// A click on another output's surface is outside too.
	tm = newTestMenu()
	other := &radialSurface{}
	tm.m.enter(tm.s, 600, 500, 1920, 1080)
	tm.m.motion(other, 600, 500)
	tm.m.button(other, ButtonLeft, true)
	tm.m.button(other, ButtonLeft, false)
	if tm.cancels != 1 {
		t.Fatal("a click on another output did not cancel")
	}
}

func TestRadialMenuNearTheEdge(t *testing.T) {
	tm := newTestMenu()
	tm.m.enter(tm.s, 5, 1075, 1920, 1080)
	if c := tm.m.cur(); c.x != 132 || c.y != 948 {
		t.Fatalf("ring at %v,%v, want it moved inside to 132,948", c.x, c.y)
	}
}

func TestRenderRadial(t *testing.T) {
	st := RadialStyle{}.withDefaults()
	const w, h = 400, 400
	buf := make([]byte, w*h*4)
	v := &radialView{
		x: 200, y: 200, style: st,
		items:  []RadialItem{{Label: "N"}, {Label: "E"}, {Label: "S", Disabled: true}, {Label: ""}},
		hover:  target{zone: zoneSlot, slot: 0},
		middle: []Span{{Text: "hi"}},
	}
	drawn := renderRadial(unsafe.Pointer(&buf[0]), w, h, w*4, 1, v)
	if want := (box{76, 76, 248, 248}); drawn != want {
		t.Errorf("drawn box = %+v, want %+v", drawn, want)
	}
	want := func(name string, x, y int, c [4]uint8) {
		t.Helper()
		a, r, g, b := pixel(buf, w*4, x, y)
		if [4]uint8{a, r, g, b} != c {
			t.Errorf("%s at %d,%d = %d,%d,%d,%d, want %v", name, x, y, a, r, g, b, c)
		}
	}
	want("hovered slot (near its outer edge)", 200, 84, [4]uint8{255, 0x45, 0x47, 0x5a})
	want("middle (below its text)", 200, 241, [4]uint8{255, 0x18, 0x18, 0x25})
	want("outside the ring", 10, 10, [4]uint8{0, 0, 0, 0})
	// Slot over background, both slightly translucent: all but opaque.
	if a, r, g, b := pixel(buf, w*4, 316, 200); a < 250 || [3]uint8{r, g, b} == [3]uint8{0x45, 0x47, 0x5a} {
		t.Errorf("east slot = %d,%d,%d,%d, want an (almost) opaque slot that isn't hovered", a, r, g, b)
	}

	// Without a view the buffer is only cleared.
	for i := range buf {
		buf[i] = 0xff
	}
	if drawn := renderRadial(unsafe.Pointer(&buf[0]), w, h, w*4, 1, nil); !drawn.empty() {
		t.Errorf("nothing drawn, but box %+v", drawn)
	}
	want("cleared", 200, 200, [4]uint8{0, 0, 0, 0})
}
