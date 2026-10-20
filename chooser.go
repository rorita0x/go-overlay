package overlay

import (
	"image/color"
	"strconv"
)

var DefaultChooserStyle = Style{
	Font:       "Sans 20",
	Background: color.NRGBA{0x1e, 0x1e, 0x2e, 0xcc},
	Padding:    16,
	Radius:     12,
}

var DefaultChooserKeyColor = MustHex("#9399b2")

var DefaultChooserHighlight = MustHex("#89b4fa55")

// Choice is one entry in a Chooser.
type Choice struct {
	Text     string
	Value    string
	Children []Choice
}

// Chooser shows a centered Popup offering choices, laid out as a row or,
// with Vertical, one per line. See SetChooser.
type Chooser struct {
	Choices []Choice

	Style     Style
	Vertical  bool
	Gap       string
	Keys      []uint32
	KeyColor  color.NRGBA
	Highlight color.NRGBA

	OnChoice func(index int, value string)
	OnCancel func()
}

// SetChooser shows c and installs its key handling, replacing OnKey.
func (o *Overlay) SetChooser(c Chooser) {
	if c.Style == (Style{}) {
		c.Style = DefaultChooserStyle
	}
	if c.Gap == "" {
		c.Gap = "   "
	}
	if c.KeyColor.A == 0 {
		c.KeyColor = DefaultChooserKeyColor
	}
	if c.Highlight.A == 0 {
		c.Highlight = DefaultChooserHighlight
	}
	cs := &chooserState{o: o, c: c, cur: chooserLayer{choices: c.Choices}}
	if c.Keys != nil {
		cs.keyIndex = buildKeyIndex(c.Keys)
	}
	o.OnKey(cs.onKey)
	cs.render()
}

// ClearChooser hides the chooser's popup and stops handling its keys.
func (o *Overlay) ClearChooser() {
	o.OnKey(nil)
	o.ClearPopup()
}

type chooserLayer struct {
	choices  []Choice
	selected int
	w, h     int
}

type chooserState struct {
	o        *Overlay
	c        Chooser
	keyIndex map[uint32]int
	stack    []chooserLayer
	cur      chooserLayer
	done     bool
}

func (cs *chooserState) label() Label {
	style := cs.c.Style
	sep := cs.c.Gap
	if cs.c.Vertical {
		style.Align = AlignLeft
		sep = "\n"
	} else {
		style.Align = AlignCenter
	}

	var spans []Span
	for i, choice := range cs.cur.choices {
		if i > 0 {
			spans = append(spans, Span{Text: sep})
		}
		var hl color.NRGBA
		if i == cs.cur.selected {
			hl = cs.c.Highlight
		}
		if tag := keyTag(cs.keyAt(i)); tag != "" {
			spans = append(spans, Span{Text: tag + " ", Color: cs.c.KeyColor, Background: hl})
		}
		spans = append(spans, Span{Text: choice.Text, Background: hl})
	}
	return Label{Spans: spans, Style: style}
}

func (cs *chooserState) render() {
	l := cs.label()
	if cs.cur.w == 0 {
		cs.cur.w, cs.cur.h = MeasureLabel(l)
	}
	cs.o.SetPopup(Popup{Width: cs.cur.w, Height: cs.cur.h}, l)
}

func (cs *chooserState) keyAt(i int) uint32 {
	if cs.keyIndex != nil {
		if i < len(cs.c.Keys) {
			return cs.c.Keys[i]
		}
		return 0
	}
	return defaultKeyAt(i)
}

func (cs *chooserState) onKey(k Key) {
	if cs.done {
		return
	}
	switch k.Sym {
	case KeyEscape:
		cs.back()
		return
	case KeyReturn:
		cs.confirm(cs.cur.selected)
		return
	}
	if cs.c.Vertical {
		switch k.Sym {
		case KeyUp:
			if cs.cur.selected > 0 {
				cs.cur.selected--
				cs.render()
			}
			return
		case KeyDown:
			if cs.cur.selected < len(cs.cur.choices)-1 {
				cs.cur.selected++
				cs.render()
			}
			return
		}
	} else {
		switch k.Sym {
		case KeyLeft:
			if cs.cur.selected > 0 {
				cs.cur.selected--
				cs.render()
			}
			return
		case KeyRight:
			if cs.cur.selected < len(cs.cur.choices)-1 {
				cs.cur.selected++
				cs.render()
			}
			return
		}
	}

	if cs.keyIndex != nil {
		if i, ok := cs.keyIndex[k.Sym]; ok {
			cs.confirm(i)
		}
		return
	}
	if i := defaultIndexFor(k.Sym); i >= 0 && i < len(cs.cur.choices) {
		cs.confirm(i)
	}
}

func (cs *chooserState) confirm(i int) {
	if i < 0 || i >= len(cs.cur.choices) {
		return
	}
	choice := cs.cur.choices[i]
	if choice.Children != nil {
		cs.stack = append(cs.stack, cs.cur)
		cs.cur = chooserLayer{choices: choice.Children}
		cs.render()
		return
	}
	cs.done = true
	value := choice.Value
	if value == "" {
		value = choice.Text
	}
	cs.o.ClearChooser()
	if cs.c.OnChoice != nil {
		cs.c.OnChoice(i, value)
	}
}

func (cs *chooserState) back() {
	if n := len(cs.stack); n > 0 {
		cs.cur = cs.stack[n-1]
		cs.stack = cs.stack[:n-1]
		cs.render()
		return
	}
	cs.done = true
	cs.o.ClearChooser()
	if cs.c.OnCancel != nil {
		cs.c.OnCancel()
	}
}

func buildKeyIndex(keys []uint32) map[uint32]int {
	idx := make(map[uint32]int, len(keys))
	for i, k := range keys {
		idx[k] = i
	}
	return idx
}

func defaultKeyAt(i int) uint32 {
	switch {
	case i == 9:
		return '0'
	case i >= 0 && i < 9:
		return uint32('1' + i)
	}
	return 0
}

func defaultIndexFor(sym uint32) int {
	switch {
	case sym == '0':
		return 9
	case sym >= '1' && sym <= '9':
		return int(sym - '1')
	}
	return -1
}

func keyTag(sym uint32) string {
	switch {
	case sym >= '0' && sym <= '9':
		return string(rune(sym))
	case sym >= 0xffbe && sym <= 0xffe0:
		return "F" + strconv.Itoa(int(sym-0xffbe+1))
	}
	return ""
}
