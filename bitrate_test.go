package vmx

import (
	"os"
	"testing"
)

func TestCalculateBitrateMatchesFloat32Reference(t *testing.T) {
	// Values from libvmx's float (32-bit) VMX_CalculateBitrate. A float64
	// implementation returns 917504/1835008/1156055/1032192/458752 for the
	// maximum targets, which changes the adaptive-quality threshold by a byte.
	cases := []struct {
		mbps int32
		min  bool
		want int32
	}{
		{400, false, 917503},
		{800, false, 1835007},
		{504, false, 1156054},
		{450, false, 1032191},
		{200, false, 458751},
		{72, true, 149422},
		{72, false, 165150},
	}
	for _, tc := range cases {
		if got := calculateBitrate(tc.mbps, tc.min); got != tc.want {
			t.Errorf("calculateBitrate(%d, %v) = %d, want %d", tc.mbps, tc.min, got, tc.want)
		}
	}
}

func TestOversizedStrideRejected(t *testing.T) {
	c, err := New(Config{Width: 64, Height: 64})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EncodeUYVY(nil, maxInt); err == nil {
		t.Fatal("oversized input stride accepted")
	}
	if err := c.DecodeUYVY(nil, maxInt); err == nil {
		t.Fatal("oversized output stride accepted")
	}
}

// TestQualityAdaptationMatchesReference pins the adaptive-quality sequence to
// vmx-rs/libvmx for the 320x180 fixture. The float32 targets matter here: with
// float64 the maximum target is one byte higher and the sequence diverges.
func TestQualityAdaptationMatchesReference(t *testing.T) {
	src, err := os.ReadFile("testdata/320x180.uyvy")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ n, q int }{
		{28480, 82}, {30626, 84}, {32840, 86}, {35193, 88},
		{37999, 90}, {41463, 92}, {46196, 93}, {49039, 94},
	}
	enc, err := New(Config{Width: 320, Height: 180, Profile: ProfileOmtSQ, ColorSpace: ColorBT709})
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range want {
		frame := append([]byte(nil), src...)
		for j := range frame {
			frame[j] += byte(i)
		}
		if err := enc.EncodeUYVY(frame, 320*2); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, enc.MaxEncodedSize())
		n, err := enc.SaveTo(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n != w.n || enc.Quality() != w.q {
			t.Fatalf("frame %d: got n=%d q=%d want n=%d q=%d", i, n, enc.Quality(), w.n, w.q)
		}
	}
}
