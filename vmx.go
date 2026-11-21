// Package vmx implements the VMX1 intra-only video codec.
package vmx

import (
	"errors"
	"fmt"
	"math/bits"
	"runtime"
	"sync"

	vmxbits "github.com/MikanseiLaboratory/vmx-go/internal/bits"
	"github.com/MikanseiLaboratory/vmx-go/internal/color"
	"github.com/MikanseiLaboratory/vmx-go/internal/dct"
	"github.com/MikanseiLaboratory/vmx-go/internal/simd"
	"github.com/MikanseiLaboratory/vmx-go/internal/tables"
)

const (
	SliceHeight  = 16
	Alignment    = 16
	MinWidth     = 16
	MinHeight    = 16
	MaxWidth     = 7680
	MaxHeight    = 4320
	MaxQ         = 98
	QualityCount = 25
)

// Profile is the VMX compression profile / quality preset.
type Profile int32

const (
	ProfileDefault Profile = 0
	ProfileLQ      Profile = 33
	ProfileSQ      Profile = 66
	ProfileHQ      Profile = 99
	ProfileOmtLQ   Profile = 133
	ProfileOmtSQ   Profile = 166
	ProfileOmtHQ   Profile = 199
)

// ColorSpace selects the RGB<->YUV matrix.
type ColorSpace int32

const (
	ColorUndefined ColorSpace = 0
	ColorBT601     ColorSpace = 601
	ColorBT709     ColorSpace = 709
)

// ImageFormat is the uncompressed input format.
type ImageFormat int32

const (
	FormatUYVY ImageFormat = iota
	FormatYUY2
	FormatNV12
	FormatYV12
	FormatBGRA
	FormatBGRX
)

// Config configures a Codec.
type Config struct {
	Width      int
	Height     int
	Profile    Profile
	ColorSpace ColorSpace
}

// ErrOutputTooSmall is returned when SaveTo's destination is too short.
type ErrOutputTooSmall struct{ Need, Have int }

func (e ErrOutputTooSmall) Error() string {
	return fmt.Sprintf("vmx: output buffer too small: need %d, have %d", e.Need, e.Have)
}

var (
	ErrInvalidConfig      = errors.New("vmx: invalid configuration")
	ErrInvalidParameters  = errors.New("vmx: invalid parameters")
	ErrBitstream          = errors.New("vmx: invalid bitstream")
	ErrUnsupported        = errors.New("vmx: unsupported format")
	ErrInvalidSliceCount  = errors.New("vmx: invalid slice count")
	ErrInvalidCodecFormat = errors.New("vmx: invalid codec format")
)

type sliceSet struct {
	dc       *vmxbits.SliceData
	ac       *vmxbits.SliceData
	offset   [4]int
	offset16 [4]int
	height   int
	lower    bool
	temp     [64]int16
}

// Codec is a VMX1 encoder and decoder. It is not safe for concurrent use.
type Codec struct {
	width, height int
	profile       Profile
	colorSpace    ColorSpace
	quality       int
	minQuality    int
	dcShift       int
	matrixIdx     int

	decodePresets [][64]uint16
	encodePresets [][192]uint16

	planes  [4][]byte
	strides [4]int
	slices  []sliceSet
	aligned int

	threads int
	format  ImageFormat
}

// New creates a Codec.
func New(cfg Config) (*Codec, error) {
	w, h := cfg.Width, cfg.Height
	if w < MinWidth || w > MaxWidth || h < MinHeight || h > MaxHeight || w%2 != 0 {
		return nil, fmt.Errorf("%w: %dx%d", ErrInvalidConfig, w, h)
	}
	profile := cfg.Profile
	if profile == ProfileDefault {
		profile = ProfileHQ
	}
	cs := cfg.ColorSpace
	if cs == ColorUndefined {
		if h >= 720 {
			cs = ColorBT709
		} else {
			cs = ColorBT601
		}
	}

	br := lookupBitrate(profile, h)
	ncpu := runtime.NumCPU()
	threads := int(br.threads)
	if h >= 4320 {
		threads = max(threads, 16)
	} else if h >= 2160 {
		threads = max(threads, 8)
	} else if h >= 1080 {
		threads = max(threads, 6)
	} else {
		threads = max(threads, 2)
	}
	threads = min(threads, ncpu)
	threads = max(threads, 1)

	yStride := alignUp(w, 8)
	uvStride := alignUp(w/2, 8)
	aligned := alignUp(h, 16)
	yLen := yStride * aligned
	uvLen := uvStride * aligned

	planes := [4][]byte{}
	planes[0] = make([]byte, yLen)
	planes[1] = make([]byte, uvLen)
	planes[2] = make([]byte, uvLen)
	planes[3] = make([]byte, yLen)
	for i := range planes[1] {
		planes[1][i] = 128
		planes[2][i] = 128
	}
	for i := range planes[3] {
		planes[3][i] = 255
	}

	sliceCount := aligned / SliceHeight
	dcCap := yStride * SliceHeight * 2
	acCap := yStride * SliceHeight * 4
	slices := make([]sliceSet, sliceCount)
	var off, off16 [4]int
	for i := 0; i < sliceCount; i++ {
		s := sliceSet{dc: vmxbits.New(dcCap), ac: vmxbits.New(acCap), height: SliceHeight}
		if i == sliceCount-1 {
			s.height = SliceHeight - (aligned - h)
		}
		mid := sliceCount / 2
		if (mid > 0 && i == mid-1) || i == sliceCount-1 {
			_ = mid
		}
		s.lower = i >= sliceCount/2
		s.offset = off
		s.offset16 = off16
		off[0] += yStride * SliceHeight
		off[1] += uvStride * SliceHeight
		off[2] += uvStride * SliceHeight
		off[3] += yStride * SliceHeight
		off16[0] += yStride * SliceHeight * 2
		off16[1] += uvStride * SliceHeight * 2
		off16[2] += uvStride * SliceHeight * 2
		off16[3] += yStride * SliceHeight * 2
		slices[i] = s
	}

	decodePresets := make([][64]uint16, QualityCount)
	encodePresets := make([][192]uint16, QualityCount)
	for qi, qs := range tables.Quality {
		var dec [64]uint16
		var enc [192]uint16
		for y := 0; y < 64; y++ {
			if y == 0 {
				dec[y] = tables.QuantMatrix[0]
			} else {
				dec[y] = tables.QuantMatrix[y] * uint16(qs)
			}
			rc := createReciprocal(dec[y])
			enc[y] = rc[0]
			enc[y+64] = rc[1]
			enc[y+128] = rc[2]
		}
		decodePresets[qi] = dec
		encodePresets[qi] = enc
	}

	c := &Codec{
		width: w, height: h, profile: profile, colorSpace: cs,
		quality: 80, minQuality: int(br.minQuality), dcShift: int(br.dcShift),
		decodePresets: decodePresets, encodePresets: encodePresets,
		planes: planes, strides: [4]int{yStride, uvStride, uvStride, yStride},
		slices: slices, aligned: aligned, threads: threads,
	}
	c.setQualityInternal(80)
	return c, nil
}

func (c *Codec) Size() (int, int)       { return c.width, c.height }
func (c *Codec) ColorSpace() ColorSpace { return c.colorSpace }
func (c *Codec) SimdPath() string       { return "scalar" }
func (c *Codec) Threads() int           { return c.threads }
func (c *Codec) SetThreads(n int) {
	if n > 0 {
		c.threads = n
	}
}
func (c *Codec) Quality() int { return c.quality }

func (c *Codec) SetColorSpace(cs ColorSpace) {
	if cs == ColorUndefined {
		if c.height >= 720 {
			cs = ColorBT709
		} else {
			cs = ColorBT601
		}
	}
	c.colorSpace = cs
}

func (c *Codec) SetQuality(q int) {
	if q > MaxQ {
		q = MaxQ
	}
	if q < c.minQuality {
		q = c.minQuality
	}
	c.setQualityInternal(q)
}

func (c *Codec) setQualityInternal(q int) {
	idx := 0
	for i, qs := range tables.Quality {
		if int(qs) >= 100-q {
			q = 100 - int(qs)
			idx = i
			break
		}
	}
	c.quality = q
	c.matrixIdx = idx
}

func (c *Codec) MaxEncodedSize() int {
	n := 5
	for i := range c.slices {
		n += 4 + c.slices[i].dc.MaxLength + 4 + c.slices[i].ac.MaxLength
	}
	return n
}

func alignUp(v, a int) int { return (v + a - 1) / a * a }

func createReciprocal(divisor uint16) [3]uint16 {
	if divisor == 1 {
		return [3]uint16{0, 1, 1}
	}
	b := (16 - bits.LeadingZeros16(divisor)) - 1
	r := 2*8 + b
	fq := (uint32(1) << uint(r)) / uint32(divisor)
	fr := (uint32(1) << uint(r)) % uint32(divisor)
	cc := uint16(divisor / 2)
	if fr == 0 {
		fq >>= 1
		r--
	} else if fr <= uint32(divisor/2) {
		cc++
	} else {
		fq++
	}
	s := uint16(1) << uint(2*8*2-r)
	return [3]uint16{cc, uint16(fq), s}
}

// EncodeUYVY encodes a packed UYVY frame.
func (c *Codec) EncodeUYVY(src []byte, stride int) error {
	if err := c.validateInput(src, stride, 2); err != nil {
		return err
	}
	c.format = FormatUYVY
	if !simd.UYVYToPlanar(src, stride, c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], c.width, c.height) {
		color.UYVYToPlanar(src, stride, c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], c.width, c.height)
	}
	c.encodeSlices(3)
	return nil
}

// EncodeYUY2 encodes a packed YUY2 frame.
func (c *Codec) EncodeYUY2(src []byte, stride int) error {
	if err := c.validateInput(src, stride, 2); err != nil {
		return err
	}
	c.format = FormatYUY2
	color.YUY2ToPlanar(src, stride, c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], c.width, c.height)
	c.encodeSlices(3)
	return nil
}

func (c *Codec) validateInput(src []byte, stride, bpp int) error {
	if stride < c.width*bpp {
		return fmt.Errorf("%w: stride %d", ErrInvalidParameters, stride)
	}
	if len(src) < stride*c.height {
		return fmt.Errorf("%w: input too small", ErrInvalidParameters)
	}
	return nil
}

func (c *Codec) validateOutput(dst []byte, stride, bpp int) error {
	if stride < c.width*bpp {
		return fmt.Errorf("%w: stride %d", ErrInvalidParameters, stride)
	}
	need := stride * c.height
	if len(dst) < need {
		return ErrOutputTooSmall{Need: need, Have: len(dst)}
	}
	return nil
}

func (c *Codec) encodeSlices(planeCount int) {
	c.parallelSlices(func(start, end int) {
		for i := start; i < end; i++ {
			s := &c.slices[i]
			s.dc.ResetEncode()
			s.ac.ResetEncode()
			for pi := 0; pi < planeCount; pi++ {
				encodePlane(c.planes[pi], c.strides[pi], s.offset[pi], s.dc, s.ac,
					c.encodePresets[c.matrixIdx][:], c.dcShift, pi, &s.temp)
			}
		}
	})
}

func (c *Codec) parallelSlices(fn func(start, end int)) {
	n := len(c.slices)
	workers := c.threads
	if workers < 1 {
		workers = 1
	}
	if workers > n {
		workers = n
	}
	if workers == 1 {
		fn(0, n)
		return
	}
	var wg sync.WaitGroup
	chunk := (n + workers - 1) / workers
	for start := 0; start < n; start += chunk {
		end := start + chunk
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(s, e int) { defer wg.Done(); fn(s, e) }(start, end)
	}
	wg.Wait()
}

func encodePlane(plane []byte, stride, offset int, dc, ac *vmxbits.SliceData, matrix []uint16, dcShift, planeIndex int, temp *[64]int16) {
	addVal := int16(0)
	if planeIndex == 0 || planeIndex == 3 {
		addVal = -128
	}
	dcRound := int16(0)
	if dcShift > 0 {
		dcRound = int16(1) << uint(dcShift-1)
	}
	var dcPred int16
	var numZeros uint32
	for y := 0; y < SliceHeight; y += 8 {
		for x := 0; x < stride; x += 8 {
			srcOff := offset + y*stride + x
			if srcOff+7*stride+8 > len(plane) {
				continue
			}
			dct.FDCTQuantZig(plane[srcOff:], stride, matrix, addVal, temp)
			dcVal := int16((int32(temp[0]) + int32(dcRound)) >> uint(dcShift))
			var mask uint64
			for i := 1; i < 64; i++ {
				if temp[i] != 0 {
					mask |= 1 << uint(i)
				}
			}
			dc.EncodeDC(int16(int32(dcVal) - int32(dcPred)))
			dc.EmitBits32()
			dcPred = dcVal
			if mask == 0 {
				numZeros += 64
				continue
			}
			pos := 0
			m := mask
			nz := bits.TrailingZeros64(m)
			numZeros += uint32(nz)
			ac.EncodeZeros(&numZeros)
			ac.EmitBits32()
			ac.EncodeValue(vmxbits.GolombSymbol(temp[nz]))
			pos = nz + 1
			m >>= uint(nz)
			m >>= 1
			ac.EmitBits32()
			for {
				if m != 0 {
					nz = bits.TrailingZeros64(m)
					ac.EncodeZerosSmall(uint64(nz))
					ac.EncodeValue(vmxbits.GolombSymbol(temp[pos+nz]))
					pos += nz + 1
					m >>= uint(nz + 1)
					ac.EmitBits32()
				} else {
					numZeros = uint32(64 - pos)
					break
				}
			}
		}
	}
	ac.EncodeZeros(&numZeros)
	ac.EmitBits32()
	ac.FlushRemainingBits()
	dc.FlushRemainingBits()
}

func (c *Codec) decodeSlices() {
	c.parallelSlices(func(start, end int) {
		for i := start; i < end; i++ {
			s := &c.slices[i]
			s.dc.ResetDecode()
			s.ac.ResetDecode()
			for pi := 0; pi < 3; pi++ {
				decodePlane(c.planes[pi], c.strides[pi], s.offset[pi], s.dc, s.ac,
					c.decodePresets[c.matrixIdx][:], c.dcShift, pi, &s.temp)
			}
		}
	})
}

func decodePlane(plane []byte, stride, offset int, dc, ac *vmxbits.SliceData, matrix []uint16, dcShift, planeIndex int, temp *[64]int16) {
	addVal := int16(0)
	if planeIndex == 0 || planeIndex == 3 {
		addVal = 128
	}
	var dcPred int16
	var terms uint64
	for y := 0; y < SliceHeight; y += 8 {
		for x := 0; x < stride; x += 8 {
			for i := range temp {
				temp[i] = 0
			}
			valid := terms < 64
			for terms < 64 {
				l := ac.PeekGolombLookup()
				if l.Length != 0 {
					ac.BitsLeft -= int(l.Length)
					if terms < 64 {
						temp[terms] = int16(l.Value)
					}
					terms += uint64(l.Zeros)
				} else {
					b := ac.GetBitB()
					if b != 0 {
						b2 := ac.GetBitB()
						if b2 != 0 {
							terms++
						} else {
							bc := ac.GetZerosB() + 2
							val := ac.GetBitsB(uint32(bc))
							terms += val
						}
					} else {
						bc := ac.GetZerosB() + 2
						val := ac.GetBitsB(uint32(bc))
						if terms < 64 {
							temp[terms] = vmxbits.GetIntFrom2MagSign(val - 1)
						}
						terms++
					}
				}
				ac.ReloadBits()
			}
			terms -= 64
			if dc.GetBit() != 0 {
				dc.GetBit()
			} else {
				bc := dc.GetZeros() + 2
				val := dc.GetBits(uint32(bc))
				temp[0] = vmxbits.GetIntFrom2MagSign(val - 1)
				temp[0] = int16(uint16(temp[0]) << uint(dcShift))
			}
			temp[0] = int16(int32(temp[0]) + int32(dcPred))
			dcPred = temp[0]
			dstOff := offset + y*stride + x
			if dstOff+7*stride+8 > len(plane) {
				continue
			}
			if valid {
				dct.ZigInvQuantIDCT(temp, matrix, plane[dstOff:], stride, addVal)
			} else {
				dct.BroadcastDC(temp[0], plane[dstOff:], stride, addVal)
			}
		}
	}
	ac.RewindOverread(terms)
	dc.FlushRemainingReadBits()
	ac.FlushRemainingReadBits()
}

// DecodeUYVY decodes to packed UYVY.
func (c *Codec) DecodeUYVY(dst []byte, stride int) error {
	if err := c.validateOutput(dst, stride, 2); err != nil {
		return err
	}
	c.decodeSlices()
	color.PlanarToUYVY(c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], dst, stride, c.width, c.height)
	return nil
}

// DecodeYUY2 decodes to packed YUY2.
func (c *Codec) DecodeYUY2(dst []byte, stride int) error {
	if err := c.validateOutput(dst, stride, 2); err != nil {
		return err
	}
	c.decodeSlices()
	color.PlanarToYUY2(c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], dst, stride, c.width, c.height)
	return nil
}

// DecodeBGRA decodes to packed BGRA.
func (c *Codec) DecodeBGRA(dst []byte, stride int) error {
	if err := c.validateOutput(dst, stride, 4); err != nil {
		return err
	}
	c.decodeSlices()
	color.PlanarToBGRA(c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], c.width, c.height, dst, stride, tables.SelectYUVRGB(int(c.colorSpace), c.height))
	return nil
}

// DecodeRGBA decodes to packed RGBA.
func (c *Codec) DecodeRGBA(dst []byte, stride int) error {
	if err := c.validateOutput(dst, stride, 4); err != nil {
		return err
	}
	c.decodeSlices()
	color.PlanarToRGBA(c.planes[0], c.planes[1], c.planes[2], c.strides[0], c.strides[1], c.strides[2], c.width, c.height, dst, stride, tables.SelectYUVRGB(int(c.colorSpace), c.height))
	return nil
}
