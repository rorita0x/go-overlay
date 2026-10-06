package overlay

import (
	"image"
	"image/color"
	"testing"
	"unsafe"
)

func TestImageContent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 5, 3))
	img.SetRGBA(0, 0, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44})
	img.SetRGBA(3, 1, color.RGBA{R: 0xff, A: 0xff})

	ct := imageContent(Image{RGBA: img, Scale: 2})
	if ct.pw != 4 || ct.ph != 2 {
		t.Fatalf("pixel size %dx%d, want 4x2", ct.pw, ct.ph)
	}
	if w, h := ct.logicalSize(); w != 2 || h != 1 {
		t.Fatalf("logical size %dx%d, want 2x1", w, h)
	}
	px := func(x, y int) uint32 { return *(*uint32)(unsafe.Pointer(&ct.pix[(y*ct.pw+x)*4])) }
	if got := px(0, 0); got != 0x44112233 {
		t.Errorf("pixel (0,0) = %#08x, want 0x44112233", got)
	}
	if got := px(3, 1); got != 0xffff0000 {
		t.Errorf("pixel (3,1) = %#08x, want 0xffff0000", got)
	}
}

func TestImageContentTinyImage(t *testing.T) {
	ct := imageContent(Image{RGBA: image.NewRGBA(image.Rect(0, 0, 1, 1)), Scale: 3})
	if w, h := ct.logicalSize(); w != 1 || h != 1 {
		t.Errorf("logical size %dx%d, want 1x1", w, h)
	}
}

func TestImageContentOpacity(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 0xc8, G: 0x64, B: 0x00, A: 0xc8})

	tests := []struct {
		opacity float64
		want    uint32
	}{
		{0, 0xc8c86400},
		{1, 0xc8c86400},
		{0.5, 0x64643200},
		{0.25, 0x32321900},
	}
	for _, tt := range tests {
		ct := imageContent(Image{RGBA: img, Opacity: tt.opacity})
		if got := *(*uint32)(unsafe.Pointer(&ct.pix[0])); got != tt.want {
			t.Errorf("opacity %v: pixel %#08x, want %#08x", tt.opacity, got, tt.want)
		}
	}
}
