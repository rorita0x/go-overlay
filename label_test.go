package overlay

import (
	"image/color"
	"testing"
)

func TestHex(t *testing.T) {
	tests := []struct {
		in      string
		want    color.NRGBA
		wantErr bool
	}{
		{"#fff", color.NRGBA{255, 255, 255, 255}, false},
		{"#f008", color.NRGBA{255, 0, 0, 0x88}, false},
		{"#ff8800", color.NRGBA{255, 0x88, 0, 255}, false},
		{"00000080", color.NRGBA{0, 0, 0, 0x80}, false},
		{"#12345", color.NRGBA{}, true},
		{"#gggggg", color.NRGBA{}, true},
		{"", color.NRGBA{}, true},
	}
	for _, tt := range tests {
		got, err := Hex(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("Hex(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("Hex(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
