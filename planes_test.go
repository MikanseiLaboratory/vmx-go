package vmx

import (
	"bytes"
	"os"
	"testing"

	"github.com/MikanseiLaboratory/vmx-go/internal/color"
)

func TestPlanesSIMD(t *testing.T) {
	for _, tc := range []struct {
		n    string
		w, h int
	}{{"64x64", 64, 64}, {"128x72", 128, 72}, {"320x180", 320, 180}} {
		src, _ := os.ReadFile("testdata/" + tc.n + ".uyvy")
		c, _ := New(Config{Width: tc.w, Height: tc.h, Profile: ProfileOmtSQ, ColorSpace: ColorBT709})
		_ = c.EncodeUYVY(src, tc.w*2)
		ys := alignUp(tc.w, 8)
		us := alignUp(tc.w/2, 8)
		ah := alignUp(tc.h, 16)
		y := make([]byte, ys*ah)
		u := make([]byte, us*ah)
		v := make([]byte, us*ah)
		for i := range u {
			u[i] = 128
			v[i] = 128
		}
		color.UYVYToPlanar(src, tc.w*2, y, u, v, ys, us, us, tc.w, tc.h)
		if !bytes.Equal(c.planes[0], y) || !bytes.Equal(c.planes[1], u) || !bytes.Equal(c.planes[2], v) {
			for pi := 0; pi < 3; pi++ {
				a := c.planes[pi]
				b := [][]byte{y, u, v}[pi]
				for i := range a {
					if i >= len(b) {
						break
					}
					if a[i] != b[i] {
						t.Fatalf("%s plane %d first diff at %d (row %d col %d): %d vs %d", tc.n, pi, i, i/len(a)*0, 0, a[i], b[i])
					}
				}
			}
			t.Fatalf("%s plane mismatch", tc.n)
		}
	}
}
