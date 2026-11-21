package vmx

import (
	"bytes"
	"os"
	"testing"
)

func TestGoldenRustBitstreams(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"64x64", 64, 64}, {"128x72", 128, 72}, {"320x180", 320, 180},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile("testdata/" + tc.name + ".uyvy")
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile("testdata/" + tc.name + ".vmx")
			if err != nil {
				t.Fatal(err)
			}
			rustOut, err := os.ReadFile("testdata/" + tc.name + ".out.uyvy")
			if err != nil {
				t.Fatal(err)
			}

			enc, err := New(Config{Width: tc.w, Height: tc.h, Profile: ProfileOmtSQ, ColorSpace: ColorBT709})
			if err != nil {
				t.Fatal(err)
			}
			if err := enc.EncodeUYVY(src, tc.w*2); err != nil {
				t.Fatal(err)
			}
			dst := make([]byte, enc.MaxEncodedSize())
			n, err := enc.SaveTo(dst)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dst[:n], want) {
				t.Fatalf("encoded bitstream differs from Rust: got %d want %d", n, len(want))
			}

			dec, err := New(Config{Width: tc.w, Height: tc.h, ColorSpace: ColorBT709})
			if err != nil {
				t.Fatal(err)
			}
			if err := dec.LoadFrom(want); err != nil {
				t.Fatal(err)
			}
			out := make([]byte, tc.w*2*tc.h)
			if err := dec.DecodeUYVY(out, tc.w*2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, rustOut) {
				t.Fatal("decoded UYVY differs from Rust reference")
			}
		})
	}
}

func BenchmarkDecode1080p(b *testing.B) {
	w, h := 1920, 1080
	src := make([]byte, w*2*h)
	for i := range src {
		src[i] = byte(i * 13)
	}
	enc, _ := New(Config{Width: w, Height: h, Profile: ProfileOmtSQ, ColorSpace: ColorBT709})
	_ = enc.EncodeUYVY(src, w*2)
	bs := make([]byte, enc.MaxEncodedSize())
	n, _ := enc.SaveTo(bs)
	dec, _ := New(Config{Width: w, Height: h, ColorSpace: ColorBT709})
	_ = dec.LoadFrom(bs[:n])
	out := make([]byte, w*2*h)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dec.LoadFrom(bs[:n])
		_ = dec.DecodeUYVY(out, w*2)
	}
}
