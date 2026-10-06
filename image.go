package overlay

import (
	"image"
	"unsafe"
)

type Image struct {
	RGBA   *image.RGBA
	Scale  int
	Margin int
}

type content struct {
	label *Label

	pix    []byte
	pw, ph int
	scale  int
	margin int
}

func labelContent(l Label) *content {
	cl := l.clone()
	return &content{label: &cl, margin: cl.Style.Margin}
}

func imageContent(img Image) *content {
	sc := max(img.Scale, 1)
	b := img.RGBA.Bounds()
	lw, lh := max(b.Dx()/sc, 1), max(b.Dy()/sc, 1)
	ct := &content{pw: lw * sc, ph: lh * sc, scale: sc, margin: img.Margin}
	ct.pix = make([]byte, ct.pw*ct.ph*4)

	cw, ch := min(ct.pw, b.Dx()), min(ct.ph, b.Dy())
	for y := range ch {
		src := img.RGBA.Pix[img.RGBA.PixOffset(b.Min.X, b.Min.Y+y):][:cw*4]
		dst := ct.pix[y*ct.pw*4:][:cw*4]
		for i := 0; i < len(src); i += 4 {
			argb := uint32(src[i+3])<<24 | uint32(src[i])<<16 | uint32(src[i+1])<<8 | uint32(src[i+2])
			*(*uint32)(unsafe.Pointer(&dst[i])) = argb
		}
	}
	return ct
}

func (ct *content) logicalSize() (w, h int) {
	if ct.label != nil {
		return measure(ct.label)
	}
	return ct.pw / ct.scale, ct.ph / ct.scale
}
