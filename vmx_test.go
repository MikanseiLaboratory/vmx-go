package vmx

import (
	"math"
	"testing"
)

func testFrame(w, h int) []byte {
	stride := w * 2
	buf := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x += 2 {
			o := y*stride + x*2
			buf[o] = byte(128 + (x*3+y*5)%60)
			buf[o+1] = byte((x*7 + y*11) % 220)
			buf[o+2] = byte(128 + (x*13+y*17)%60)
			buf[o+3] = byte((x*5 + y*3 + 40) % 220)
		}
	}
	return buf
}

func TestRoundTripUYVY(t *testing.T) {
	for _, sz := range [][2]int{{64, 64}, {128, 72}, {320, 180}, {1920, 1080}} {
		w, h := sz[0], sz[1]
		src := testFrame(w, h)
		enc, err := New(Config{Width: w, Height: h, Profile: ProfileOmtSQ, ColorSpace: ColorBT709})
		if err != nil {
			t.Fatal(err)
		}
		if err := enc.EncodeUYVY(src, w*2); err != nil {
			t.Fatal(err)
		}
		bs := make([]byte, enc.MaxEncodedSize())
		n, err := enc.SaveTo(bs)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%dx%d encoded %d bytes", w, h, n)
		dec, err := New(Config{Width: w, Height: h, ColorSpace: ColorBT709})
		if err != nil {
			t.Fatal(err)
		}
		if err := dec.LoadFrom(bs[:n]); err != nil {
			t.Fatal(err)
		}
		out := make([]byte, w*2*h)
		if err := dec.DecodeUYVY(out, w*2); err != nil {
			t.Fatal(err)
		}
		var se int64
		for i := range src {
			d := int64(src[i]) - int64(out[i])
			se += d * d
		}
		mse := float64(se) / float64(len(src))
		psnr := 99.0
		if mse > 0 {
			psnr = 10 * math.Log10(255*255/mse)
		}
		t.Logf("psnr %.2f", psnr)
		if psnr < 25 {
			t.Fatalf("bad psnr %f", psnr)
		}
	}
}
