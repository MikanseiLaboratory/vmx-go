//go:build goexperiment.simd && amd64

package simd

import "simd/archsimd"

var (
	yIdx = [16]int8{1, 3, 5, 7, 9, 11, 13, 15}
	uIdx = [16]int8{0, 4, 8, 12}
	vIdx = [16]int8{2, 6, 10, 14}
)

// UYVYToPlanar uses AVX2 byte permutes for the 8-pixel body.
func UYVYToPlanar(src []byte, stride int, y, u, v []byte, ys, us, vs, width, height int) bool {
	if !archsimd.X86.AVX2() || width < 8 {
		return false
	}
	iy := archsimd.LoadInt8x16Array(&yIdx)
	iu := archsimd.LoadInt8x16Array(&uIdx)
	iv := archsimd.LoadInt8x16Array(&vIdx)
	var tmp [16]byte
	for row := 0; row < height; row++ {
		s := src[row*stride:]
		yd := y[row*ys:]
		ud := u[row*us:]
		vd := v[row*vs:]
		px := 0
		for ; px+8 <= width; px += 8 {
			x := archsimd.LoadUint8x16(s[px*2 : px*2+16])
			x.PermuteOrZero(iy).StoreArray(&tmp)
			copy(yd[px:px+8], tmp[:8])
			x.PermuteOrZero(iu).StoreArray(&tmp)
			copy(ud[px/2:px/2+4], tmp[:4])
			x.PermuteOrZero(iv).StoreArray(&tmp)
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

func ColorPath() string { return "avx2" }
