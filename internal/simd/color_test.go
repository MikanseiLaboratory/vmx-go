//go:build goexperiment.simd && (amd64 || arm64)

package simd

import (
	"bytes"
	"testing"

	"github.com/MikanseiLaboratory/vmx-go/internal/color"
)

func TestUYVYToPlanarSIMD(t *testing.T) {
	for _, sz := range [][2]int{{64, 4}, {64, 64}, {128, 72}, {320, 180}} {
		w, h := sz[0], sz[1]
		stride := w * 2
		src := make([]byte, stride*h)
		for i := range src {
			src[i] = byte(i*7 + 3)
		}
		y1 := make([]byte, w*h)
		u1 := make([]byte, w/2*h)
		v1 := make([]byte, w/2*h)
		y2 := make([]byte, w*h)
		u2 := make([]byte, w/2*h)
		v2 := make([]byte, w/2*h)
		if !UYVYToPlanar(src, stride, y1, u1, v1, w, w/2, w/2, w, h) {
			t.Fatal("simd unavailable")
		}
		color.UYVYToPlanar(src, stride, y2, u2, v2, w, w/2, w/2, w, h)
		if !bytes.Equal(y1, y2) || !bytes.Equal(u1, u2) || !bytes.Equal(v1, v2) {
			t.Logf("y %v/%v", y1[:16], y2[:16])
			t.Logf("u %v/%v", u1[:8], u2[:8])
			t.Logf("v %v/%v", v1[:8], v2[:8])
			t.Fatalf("%dx%d mismatch", w, h)
		}
	}
}
