//go:build goexperiment.simd && arm64

package simd

import "simd/archsimd"

var (
	yIdx = [16]uint8{1, 3, 5, 7, 9, 11, 13, 15}
	uIdx = [16]uint8{0, 4, 8, 12}
	vIdx = [16]uint8{2, 6, 10, 14}
)

// UYVYToPlanar uses NEON byte-table lookups for the 8-pixel body.
func UYVYToPlanar(src []byte, stride int, y, u, v []byte, ys, us, vs, width, height int) bool {
	if width < 8 {
		return false
	}
	iy := archsimd.LoadUint8x16Array(&yIdx)
	iu := archsimd.LoadUint8x16Array(&uIdx)
	iv := archsimd.LoadUint8x16Array(&vIdx)
	var tmp [16]byte
	for row := 0; row < height; row++ {
		s := src[row*stride:]
		yd := y[row*ys:]
		ud := u[row*us:]
		vd := v[row*vs:]
		px := 0
		for ; px+8 <= width; px += 8 {
			x := archsimd.LoadUint8x16(s[px*2 : px*2+16])
			x.LookupOrZero(iy).StoreArray(&tmp)
			copy(yd[px:px+8], tmp[:8])
			x.LookupOrZero(iu).StoreArray(&tmp)
			copy(ud[px/2:px/2+4], tmp[:4])
			x.LookupOrZero(iv).StoreArray(&tmp)
			copy(vd[px/2:px/2+4], tmp[:4])
		}
		for ; px+1 < width; px += 2 {
			ud[px/2] = s[px*2]
			yd[px] = s[px*2+1]
			vd[px/2] = s[px*2+2]
			yd[px+1] = s[px*2+3]
		}
	}
	return true
}

func ColorPath() string { return "neon" }
