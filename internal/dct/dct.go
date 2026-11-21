// Package dct implements the scalar VMX 8x8 FDCT / IDCT and quantization.
package dct

import "github.com/MikanseiLaboratory/vmx-go/internal/tables"

func SatAdd16(a, b int16) int16 {
	s := int32(a) + int32(b)
	if s > 32767 {
		return 32767
	}
	if s < -32768 {
		return -32768
	}
	return int16(s)
}

func SatSub16(a, b int16) int16 {
	s := int32(a) - int32(b)
	if s > 32767 {
		return 32767
	}
	if s < -32768 {
		return -32768
	}
	return int16(s)
}

func Mulhi16(a, b int16) int16    { return int16((int32(a) * int32(b)) >> 16) }
func MulhiU16(a, b uint16) uint16 { return uint16((uint32(a) * uint32(b)) >> 16) }

func shl16(v int16, n uint) int16 { return int16(uint16(v) << n) }

func packs32(a, b int32) (int16, int16) {
	clamp := func(v int32) int16 {
		if v < -32768 {
			return -32768
		}
		if v > 32767 {
			return 32767
		}
		return int16(v)
	}
	return clamp(a), clamp(b)
}

func fdctRow(in [8]int16, ftab *[32]int16) [8]int16 {
	s0 := SatAdd16(in[0], in[7])
	d0 := SatSub16(in[0], in[7])
	s1 := SatAdd16(in[1], in[6])
	d1 := SatSub16(in[1], in[6])
	s2 := SatAdd16(in[2], in[5])
	d2 := SatSub16(in[2], in[5])
	s3 := SatAdd16(in[3], in[4])
	d3 := SatSub16(in[3], in[4])

	full := [8]int16{s0, s1, d0, d1, s2, s3, d2, d3}
	shuf := [8]int16{s2, s3, d2, d3, s0, s1, d0, d1}

	madd := func(v [8]int16, f []int16) [4]int32 {
		return [4]int32{
			int32(v[0])*int32(f[0]) + int32(v[1])*int32(f[1]),
			int32(v[2])*int32(f[2]) + int32(v[3])*int32(f[3]),
			int32(v[4])*int32(f[4]) + int32(v[5])*int32(f[5]),
			int32(v[6])*int32(f[6]) + int32(v[7])*int32(f[7]),
		}
	}
	temp4 := madd(full, ftab[0:8])
	temp1 := madd(shuf, ftab[8:16])
	temp2 := madd(full, ftab[16:24])
	temp3 := madd(shuf, ftab[24:32])

	var lo, hi [4]int32
	for i := 0; i < 4; i++ {
		lo[i] = (temp4[i] + temp1[i] + tables.RndFrwRow) >> tables.ShiftFrwRow
		hi[i] = (temp3[i] + temp2[i] + tables.RndFrwRow) >> tables.ShiftFrwRow
	}
	o0, o1 := packs32(lo[0], lo[1])
	o2, o3 := packs32(lo[2], lo[3])
	o4, o5 := packs32(hi[0], hi[1])
	o6, o7 := packs32(hi[2], hi[3])
	return [8]int16{o0, o1, o2, o3, o4, o5, o6, o7}
}

func fdctColumnStage(in [8][8]int16) [8][8]int16 {
	var cols [8][8]int16
	for x := 0; x < 8; x++ {
		var c [8]int16
		for y := 0; y < 8; y++ {
			c[y] = in[y][x]
		}
		xmm0, xmm2, xmm7, xmm5 := c[0], c[2], c[7], c[5]
		xmm3 := xmm0
		xmm4 := xmm2
		xmm0 = SatSub16(xmm0, xmm7)
		xmm7 = SatAdd16(xmm7, xmm3)
		xmm2 = SatSub16(xmm2, xmm5)
		xmm5 = SatAdd16(xmm5, xmm4)

		xmm3, xmm4 = c[3], c[4]
		xmm1 := xmm3
		xmm3 = SatSub16(xmm3, xmm4)
		xmm4 = SatAdd16(xmm4, xmm1)

		xmm6, xmm1 := c[6], c[1]
		tmp := xmm1
		xmm1 = SatSub16(xmm1, xmm6)
		xmm6 = SatAdd16(xmm6, tmp)

		tm03 := SatSub16(xmm7, xmm4)
		tm12 := SatSub16(xmm6, xmm5)
		xmm4 = SatAdd16(xmm4, xmm4)
		xmm5 = SatAdd16(xmm5, xmm5)
		tp03 := SatAdd16(xmm4, tm03)
		tp12 := SatAdd16(xmm5, tm12)

		xmm2 = shl16(xmm2, tables.ShiftFrwCol+1)
		xmm1 = shl16(xmm1, tables.ShiftFrwCol+1)
		tp03 = shl16(tp03, tables.ShiftFrwCol)
		tp12 = shl16(tp12, tables.ShiftFrwCol)
		tm03 = shl16(tm03, tables.ShiftFrwCol)
		tm12 = shl16(tm12, tables.ShiftFrwCol)
		xmm3 = shl16(xmm3, tables.ShiftFrwCol)
		xmm0 = shl16(xmm0, tables.ShiftFrwCol)

		in4 := SatSub16(tp03, tp12)
		diff := SatSub16(xmm1, xmm2)
		tp12 = SatAdd16(tp12, tp12)
		xmm2 = SatAdd16(xmm2, xmm2)
		in0 := SatAdd16(tp12, in4)
		sum := SatAdd16(xmm2, diff)

		tmp1 := Mulhi16(tables.FDCTTan2, tm03)
		in6 := SatSub16(tmp1, tm12)
		tmp2 := Mulhi16(tables.FDCTTan2, tm12)
		in2 := SatAdd16(tmp2, tm03)

		tp65 := Mulhi16(sum, tables.FDCTSqrt2)
		in2 |= tables.FDCTRound1
		in6 |= tables.FDCTRound1
		tm65 := Mulhi16(diff, tables.FDCTSqrt2)
		tp65 |= tables.FDCTRound1

		tm465 := SatSub16(xmm3, tm65)
		tm765 := SatSub16(xmm0, tp65)
		tp765 := SatAdd16(tp65, xmm0)
		tp465 := SatAdd16(tm65, xmm3)

		tmp3 := Mulhi16(tm465, tables.FDCTTan3)
		tmp4 := Mulhi16(tp465, tables.FDCTTan1)
		tmp3 = SatAdd16(tmp3, tm465)
		tmp5 := Mulhi16(tm765, tables.FDCTTan3)
		tmp5 = SatAdd16(tmp5, tm765)
		tmp6 := Mulhi16(tp765, tables.FDCTTan1)

		in1 := SatAdd16(tmp4, tp765)
		in3 := SatSub16(tm765, tmp3)
		in5 := SatAdd16(tm465, tmp5)
		in7 := SatSub16(tmp6, tp465)

		cols[0][x] = in0
		cols[1][x] = in1
		cols[2][x] = in2
		cols[3][x] = in3
		cols[4][x] = in4
		cols[5][x] = in5
		cols[6][x] = in6
		cols[7][x] = in7
	}
	return cols
}

// FDCTQuantZig computes FDCT, quantizes in spatial order and writes zigzag order.
func FDCTQuantZig(src []byte, stride int, encodeMatrix []uint16, addVal int16, out *[64]int16) {
	var rows [8][8]int16
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			rows[y][x] = SatAdd16(int16(src[y*stride+x]), addVal)
		}
	}
	cols := fdctColumnStage(rows)
	ftabs := [4]*[32]int16{&tables.FTAB1, &tables.FTAB2, &tables.FTAB3, &tables.FTAB4}
	order := [8]int{0, 1, 2, 3, 0, 3, 2, 1}
	var spatial [64]int16
	for y, fi := range order {
		rowOut := fdctRow(cols[y], ftabs[fi])
		for x := 0; x < 8; x++ {
			spatial[y*8+x] = rowOut[x]
		}
	}
	for i := 0; i < 64; i++ {
		out[tables.ZigzagInv[i]] = spatialQuant(spatial[i], encodeMatrix, i)
	}
}

func spatialQuant(v int16, m []uint16, i int) int16 {
	if v == 0 {
		return 0
	}
	absV := uint16(-int32(v))
	if v > 0 {
		absV = uint16(v)
	}
	q := uint32(absV) + uint32(m[i])
	q = uint32(MulhiU16(uint16(q), m[i+64]))
	q = uint32(MulhiU16(uint16(q), m[i+128]))
	if v < 0 {
		return -int16(q)
	}
	return int16(q)
}

func idctRow(x [8]int16, tab *[32]int16) [8]int16 {
	madd := func(a int16, ta int16, b int16, tb int16) int32 {
		return int32(a)*int32(ta) + int32(b)*int32(tb)
	}
	var out [8]int16
	for i := 0; i < 4; i++ {
		even := madd(x[0], tab[2*i], x[2], tab[2*i+1]) + tables.IRndInvRow +
			madd(x[4], tab[8+2*i], x[6], tab[8+2*i+1])
		odd := madd(x[5], tab[24+2*i], x[7], tab[24+2*i+1]) +
			madd(x[1], tab[16+2*i], x[3], tab[16+2*i+1])
		out[i] = satI32ToI16((even + odd) >> tables.ShiftInvRow)
		out[7-i] = satI32ToI16((even - odd) >> tables.ShiftInvRow)
	}
	return out
}

func satI32ToI16(v int32) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

func idctColumn(col [8]int16, addVal int16) [8]int16 {
	r0, r1, r2, r3, r4, r5, r6, r7 := col[0], col[1], col[2], col[3], col[4], col[5], col[6], col[7]

	x0 := SatAdd16(Mulhi16(r5, tables.IDCTTG3), r5)
	x1 := SatAdd16(Mulhi16(r3, tables.IDCTTG3), r3)
	x0 = SatAdd16(x0, r3)
	x2 := SatSub16(r5, x1)
	x5 := SatSub16(Mulhi16(r1, tables.IDCTTG1), r7)
	x4 := SatAdd16(Mulhi16(r7, tables.IDCTTG1), r1)

	temp7 := SatAdd16(SatAdd16(x0, x4), 1)
	t4 := SatSub16(x4, x0)
	t5 := SatAdd16(SatSub16(x5, x2), 1)
	temp3 := SatAdd16(x5, x2)

	s := SatAdd16(t4, t5)
	d := SatSub16(t4, t5)
	m4 := SatAdd16(s, Mulhi16(tables.IDCTCos4, s)) | 1
	m0 := SatAdd16(Mulhi16(tables.IDCTCos4, d), d) | 1

	e7 := SatAdd16(Mulhi16(r6, tables.IDCTTG2), r2)
	e3 := SatSub16(Mulhi16(r2, tables.IDCTTG2), r6)
	sum04 := SatAdd16(r4, r0)
	dif04 := SatSub16(r0, r4)

	rndCol := int16(tables.IRndInvCol)
	rndCorr := int16(tables.IRndInvCorr)
	b0 := SatAdd16(SatAdd16(sum04, e7), rndCol)
	b3 := SatAdd16(SatSub16(sum04, e7), rndCorr)
	b1 := SatAdd16(SatAdd16(dif04, e3), rndCol)
	b2 := SatAdd16(SatSub16(dif04, e3), rndCorr)

	fin := func(v int16) int16 { return SatAdd16(v>>tables.ShiftInvCol, addVal) }
	return [8]int16{
		fin(SatAdd16(temp7, b0)),
		fin(SatAdd16(b1, m4)),
		fin(SatAdd16(b2, m0)),
		fin(SatAdd16(temp3, b3)),
		fin(SatSub16(b3, temp3)),
		fin(SatSub16(b2, m0)),
		fin(SatSub16(b1, m4)),
		fin(SatSub16(b0, temp7)),
	}
}

// ZigInvQuantIDCT dequantizes, inverse-zigzags and IDCTs one block.
func ZigInvQuantIDCT(coeffs *[64]int16, decodeMatrix []uint16, dst []byte, stride int, addVal int16) {
	var rows [8][8]int16
	for i := 0; i < 64; i++ {
		c := coeffs[tables.ZigzagInv[i]]
		rows[i/8][i%8] = int16(c*int16(decodeMatrix[i])) >> 4
	}
	for y := 0; y < 8; y++ {
		rows[y] = idctRow(rows[y], tables.IDCTRowTables[y])
	}
	for x := 0; x < 8; x++ {
		col := [8]int16{rows[0][x], rows[1][x], rows[2][x], rows[3][x], rows[4][x], rows[5][x], rows[6][x], rows[7][x]}
		out := idctColumn(col, addVal)
		for y := 0; y < 8; y++ {
			v := out[y]
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[y*stride+x] = byte(v)
		}
	}
}

func BroadcastDC(dc int16, dst []byte, stride int, addVal int16) {
	v := SatAdd16((dc+4)>>3, addVal)
	if v < 0 {
		v = 0
	} else if v > 255 {
		v = 255
	}
	pix := byte(v)
	for y := 0; y < 8; y++ {
		row := dst[y*stride : y*stride+8]
		for i := range row {
			row[i] = pix
		}
	}
}
