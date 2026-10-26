package scanner

import (
	"github.com/mzyy94/airscap/internal/vens"
	"image"
	"image/color"
	"testing"
)

func TestIX500PDFOrientation(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  int
	}{
		{"FUJITSU ScanSnap iX500", 180}, {"ScanSnap iX500  ", 180},
		{"ScanSnap iX1600", 0}, {"ScanSnap iX5000", 0}, {"", 0},
	} {
		s := &Scanner{deviceName: tc.model}
		pages := []vens.Page{{Side: 0, JPEG: []byte{1, 2}}, {Side: 1, JPEG: []byte{3, 4}}}
		s.correctPDFOrientation(pages)
		s.correctPDFOrientation(pages)
		for _, p := range pages {
			if p.PDFRotation != tc.want {
				t.Fatalf("%s side %d: %d", tc.model, p.Side, p.PDFRotation)
			}
		}
		if pages[0].JPEG[0] != 1 || pages[1].JPEG[0] != 3 {
			t.Fatal("encoded images changed")
		}
	}
}

func TestRotateImage180(t *testing.T) {
	src := image.NewNRGBA(image.Rect(5, 7, 7, 10))
	src.Set(5, 7, color.NRGBA{R: 255, A: 255})
	src.Set(6, 9, color.NRGBA{B: 255, A: 255})
	got := rotateImage180(src)
	if got.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatal(got.Bounds())
	}
	if got.At(1, 2) != (color.NRGBA{R: 255, A: 255}) || got.At(0, 0) != (color.NRGBA{B: 255, A: 255}) {
		t.Fatal("corners not rotated")
	}
}
