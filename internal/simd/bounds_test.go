//go:build goexperiment.simd && (amd64 || arm64)

package simd

import "testing"

func TestBounds(t *testing.T) {
	w, h := 128, 72
	ys := 128
	us := 64
	ah := 80
	src := make([]byte, w*2*h)
	for i := range src {
		src[i] = byte(i)
	}
	y := make([]byte, ys*ah)
	u := make([]byte, us*ah)
	v := make([]byte, us*ah)
	for i := range y {
		y[i] = 0xAA
	}
	for i := range u {
		u[i] = 0xAA
	}
	for i := range v {
		v[i] = 0xAA
	}
	if !UYVYToPlanar(src, w*2, y, u, v, ys, us, us, w, h) {
		t.Fatal("no")
	}
	for i := w * h; i < len(y); i++ {
		if y[i] != 0xAA {
			t.Fatalf("y padding write at %d: %d", i, y[i])
		}
	}
	for i := w / 2 * h; i < len(u); i++ {
		if u[i] != 0xAA {
			t.Fatalf("u padding write at %d: %d", i, u[i])
		}
	}
}
