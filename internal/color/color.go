// Package color implements the packed 4:2:2 <-> planar conversions VMX needs.
package color

// UYVYToPlanar converts packed UYVY to Y, U and V planes.
func UYVYToPlanar(src []byte, stride int, y, u, v []byte, yStride, uStride, vStride, width, height int) {
	for row := 0; row < height; row++ {
		s := src[row*stride:]
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		x := 0
		px := 0
		for px+1 < width {
			ud[x] = s[px*2]
			yd[px] = s[px*2+1]
			vd[x] = s[px*2+2]
			yd[px+1] = s[px*2+3]
			x++
			px += 2
		}
	}
}

// YUY2ToPlanar converts packed YUY2 to Y, U and V planes.
func YUY2ToPlanar(src []byte, stride int, y, u, v []byte, yStride, uStride, vStride, width, height int) {
	for row := 0; row < height; row++ {
		s := src[row*stride:]
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		x := 0
		px := 0
		for px+1 < width {
			yd[px] = s[px*2]
			ud[x] = s[px*2+1]
			yd[px+1] = s[px*2+2]
			vd[x] = s[px*2+3]
			x++
			px += 2
		}
	}
}

// PlanarToUYVY packs Y, U and V planes into UYVY.
func PlanarToUYVY(y, u, v []byte, yStride, uStride, vStride int, dst []byte, stride, width, height int) {
	for row := 0; row < height; row++ {
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		d := dst[row*stride:]
		x := 0
		px := 0
		for px+1 < width {
			d[px*2] = ud[x]
			d[px*2+1] = yd[px]
			d[px*2+2] = vd[x]
			d[px*2+3] = yd[px+1]
			x++
			px += 2
		}
	}
}

// PlanarToYUY2 packs Y, U and V planes into YUY2.
func PlanarToYUY2(y, u, v []byte, yStride, uStride, vStride int, dst []byte, stride, width, height int) {
	for row := 0; row < height; row++ {
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		d := dst[row*stride:]
		x := 0
		px := 0
		for px+1 < width {
			d[px*2] = yd[px]
			d[px*2+1] = ud[x]
			d[px*2+2] = yd[px+1]
			d[px*2+3] = vd[x]
			x++
			px += 2
		}
	}
}

func mulhi16(a, b int16) int16 { return int16((int32(a) * int32(b)) >> 16) }

func satAdd16(a, b int16) int16 {
	s := int32(a) + int32(b)
	if s > 32767 {
		return 32767
	}
	if s < -32768 {
		return -32768
	}
	return int16(s)
}

func satSub16(a, b int16) int16 {
	s := int32(a) - int32(b)
	if s > 32767 {
		return 32767
	}
	if s < -32768 {
		return -32768
	}
	return int16(s)
}

func yuvPixel(yy byte, cb, cr int16, table *[5]int16) (b, g, r byte) {
	ySat := int16(yy)
	if ySat > 16 {
		ySat -= 16
	} else {
		ySat = 0
	}
	y0 := mulhi16(ySat<<6, table[0])
	r16 := satAdd16(y0, mulhi16(cr<<6, table[1]))
	b16 := satAdd16(y0, mulhi16(cb<<7, table[4]))
	g16 := satSub16(satSub16(y0, mulhi16(cb<<6, table[2])), mulhi16(cr<<6, table[3]))
	r16 = satAdd16(r16, 8) >> 4
	g16 = satAdd16(g16, 8) >> 4
	b16 = satAdd16(b16, 8) >> 4
	clamp := func(v int16) byte {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return byte(v)
	}
	return clamp(b16), clamp(g16), clamp(r16)
}

// PlanarToRGBA converts 4:2:2 planes to packed RGBA8.
func PlanarToRGBA(y, u, v []byte, yStride, uStride, vStride int, width, height int, dst []byte, dstStride int, table *[5]int16) {
	for row := 0; row < height; row++ {
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		d := dst[row*dstStride:]
		x := 0
		px := 0
		for px+1 < width {
			cb := int16(ud[x]) - 128
			cr := int16(vd[x]) - 128
			b0, g0, r0 := yuvPixel(yd[px], cb, cr, table)
			b1, g1, r1 := yuvPixel(yd[px+1], cb, cr, table)
			o := px * 4
			d[o], d[o+1], d[o+2], d[o+3] = r0, g0, b0, 255
			d[o+4], d[o+5], d[o+6], d[o+7] = r1, g1, b1, 255
			x++
			px += 2
		}
	}
}

// PlanarToBGRA converts 4:2:2 planes to packed BGRA8.
func PlanarToBGRA(y, u, v []byte, yStride, uStride, vStride int, width, height int, dst []byte, dstStride int, table *[5]int16) {
	for row := 0; row < height; row++ {
		yd := y[row*yStride:]
		ud := u[row*uStride:]
		vd := v[row*vStride:]
		d := dst[row*dstStride:]
		x := 0
		px := 0
		for px+1 < width {
			cb := int16(ud[x]) - 128
			cr := int16(vd[x]) - 128
			b0, g0, r0 := yuvPixel(yd[px], cb, cr, table)
			b1, g1, r1 := yuvPixel(yd[px+1], cb, cr, table)
			o := px * 4
			d[o], d[o+1], d[o+2], d[o+3] = b0, g0, r0, 255
			d[o+4], d[o+5], d[o+6], d[o+7] = b1, g1, r1, 255
			x++
			px += 2
		}
	}
}
