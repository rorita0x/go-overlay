package overlay

import (
	"reflect"
	"testing"
)

// recorder is a pointerTarget that writes down what it gets.
type recorder struct{ events []string }

func (r *recorder) cursor() uint32             { return 0 }
func (r *recorder) pointerEnter(x, y float64)  { r.events = append(r.events, "enter") }
func (r *recorder) pointerMotion(x, y float64) { r.events = append(r.events, "motion") }
func (r *recorder) pointerLeave()              { r.events = append(r.events, "leave") }
func (r *recorder) pointerButton(b Button, pressed bool) {
	if pressed {
		r.events = append(r.events, "press")
	} else {
		r.events = append(r.events, "release")
	}
}

func TestPointerRouting(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	o := &Overlay{pointerTargets: map[uintptr]pointerTarget{1: a, 2: b}}

	o.pointerEnter(1, 5, 5)
	o.pointerMotion(6, 6)
	o.pointerButton(uint32(ButtonLeft), 1)
	o.pointerButton(uint32(ButtonLeft), 0)
	o.pointerLeave(2) // not the focused surface: ignored
	o.pointerLeave(1)
	o.pointerMotion(7, 7) // no focus: goes nowhere
	o.pointerEnter(3, 0, 0)
	o.pointerButton(uint32(ButtonLeft), 1) // a surface that isn't ours

	if want := []string{"enter", "motion", "press", "release", "leave"}; !reflect.DeepEqual(a.events, want) {
		t.Errorf("a got %v, want %v", a.events, want)
	}
	if len(b.events) != 0 {
		t.Errorf("b got %v, want nothing", b.events)
	}
}

func TestClickableLabel(t *testing.T) {
	var clicks []Click
	s := &surface{content: labelContent(Label{OnClick: func(c Click) { clicks = append(clicks, c) }})}

	s.pointerEnter(3, 4)
	s.pointerButton(ButtonLeft, true)
	s.pointerMotion(10, 12)
	s.pointerButton(ButtonLeft, false) // a click, where it was released
	s.pointerButton(ButtonRight, true)
	s.pointerLeave()
	s.pointerEnter(1, 1)
	s.pointerButton(ButtonRight, false)  // pressed before leaving: no click
	s.pointerButton(ButtonMiddle, false) // released without a press: no click

	if want := []Click{{Button: ButtonLeft, X: 10, Y: 12}}; !reflect.DeepEqual(clicks, want) {
		t.Errorf("clicks = %+v, want %+v", clicks, want)
	}

	plain := labelContent(Label{})
	if plain.clickable() || !s.content.clickable() {
		t.Error("only a label with OnClick should be clickable")
	}
}
