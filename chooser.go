package overlay

import "image/color"

// DefaultChooserStyle is used by SetChooser when Chooser.Style is a zero Style.
var DefaultChooserStyle = Style{
	Font:       "Sans 20",
	Background: color.NRGBA{0x1e, 0x1e, 0x2e, 0xcc},
	Padding:    16,
	Radius:     12,
}

// DefaultChooserDigit is used when Chooser.Digit is a zero color.
var DefaultChooserDigit = MustHex("#9399b2")

// DefaultChooserHighlight is used when Chooser.Highlight is a zero color.
var DefaultChooserHighlight = MustHex("#89b4fa55")

// Chooser shows a centered Popup offering up to 10 choices, each tagged with
// a digit (1-9 then 0). Digit keys confirm that choice immediately; Left and
// Right move a highlighted cursor, clamped at both ends; Enter confirms the
// cursor; Escape cancels. See SetChooser.
type Chooser struct {
	Choices []string

	Style     Style // Align is forced to AlignCenter; zero Style means DefaultChooserStyle
	Gap       string
	Digit     color.NRGBA
	Highlight color.NRGBA

	OnChoice func(index int, choice string)
	OnCancel func()
}

// SetChooser shows c with its cursor on the first choice and installs the key
// handling for it, replacing any handler set via OnKey. OnChoice fires once,
// for a digit press or Enter; OnCancel fires once, for Escape. Neither fires
// more than once per SetChooser call. Safe to call from any goroutine.
func (o *Overlay) SetChooser(c Chooser) {
	if c.Style == (Style{}) {
		c.Style = DefaultChooserStyle
	}
	if c.Gap == "" {
		c.Gap = "   "
	}
	if c.Digit.A == 0 {
		c.Digit = DefaultChooserDigit
	}
	if c.Highlight.A == 0 {
		c.Highlight = DefaultChooserHighlight
	}
	cs := &chooserState{o: o, c: c}
	o.OnKey(cs.onKey)
	cs.render()
}

// ClearChooser hides the chooser's popup and stops handling its keys.
func (o *Overlay) ClearChooser() {
	o.OnKey(nil)
	o.ClearPopup()
}

type chooserState struct {
	o        *Overlay
	c        Chooser
	selected int
	done     bool
	w, h     int
}

func (cs *chooserState) label() Label {
	style := cs.c.Style
	style.Align = AlignCenter

	var spans []Span
	for i, choice := range cs.c.Choices {
		if i > 0 {
			spans = append(spans, Span{Text: cs.c.Gap})
		}
		var hl color.NRGBA
		if i == cs.selected {
			hl = cs.c.Highlight
		}
		spans = append(spans,
			Span{Text: string(digitFor(i)) + " ", Color: cs.c.Digit, Background: hl},
			Span{Text: choice, Background: hl},
		)
	}
	return Label{Spans: spans, Style: style}
}

func (cs *chooserState) render() {
	l := cs.label()
	if cs.w == 0 {
		cs.w, cs.h = MeasureLabel(l)
	}
	cs.o.SetPopup(Popup{Width: cs.w, Height: cs.h}, l)
}

func (cs *chooserState) onKey(k Key) {
	if cs.done {
		return
	}
	switch k.Sym {
	case KeyEscape:
		cs.cancel()
		return
	case KeyReturn:
		cs.confirm(cs.selected)
		return
	case KeyLeft:
		if cs.selected > 0 {
			cs.selected--
			cs.render()
		}
		return
	case KeyRight:
		if cs.selected < len(cs.c.Choices)-1 {
			cs.selected++
			cs.render()
		}
		return
	}
	if i := indexForDigit(k.Rune); i >= 0 {
		cs.confirm(i)
	}
}

func (cs *chooserState) confirm(i int) {
	if i < 0 || i >= len(cs.c.Choices) {
		return
	}
	cs.done = true
	choice := cs.c.Choices[i]
	cs.o.ClearChooser()
	if cs.c.OnChoice != nil {
		cs.c.OnChoice(i, choice)
	}
}

func (cs *chooserState) cancel() {
	cs.done = true
	cs.o.ClearChooser()
	if cs.c.OnCancel != nil {
		cs.c.OnCancel()
	}
}

func digitFor(i int) rune {
	if i == 9 {
		return '0'
	}
	return rune('1' + i)
}

func indexForDigit(r rune) int {
	switch {
	case r == '0':
		return 9
	case r >= '1' && r <= '9':
		return int(r - '1')
	}
	return -1
}
