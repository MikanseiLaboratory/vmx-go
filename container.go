package vmx

import (
	"encoding/binary"
	"fmt"
)

const (
	codecFormatNone        = 0
	codecFormatProgressive = 1
	codecFormatInterlaced  = 2
	codecFormatExtended    = 3
)

type frameHeader struct {
	format     ImageFormat
	quality    int
	sliceCount int
	dcShift    int
}

// SaveTo writes the encoded VMX1 bitstream into dst.
func (c *Codec) SaveTo(dst []byte) (int, error) {
	need := c.encodedLength()
	if len(dst) < need {
		return 0, ErrOutputTooSmall{Need: need, Have: len(dst)}
	}
	n, err := saveTo(dst, c.slices, c.format, c.quality, c.dcShift)
	if err != nil {
		return 0, err
	}
	c.adjustBitrate(n)
	return n, nil
}

func (c *Codec) adjustBitrate(frameLen int) {
	br := lookupBitrate(c.profile, c.height)
	adjustBitrate(&c.quality, int(br.minQuality), frameLen, br.targetBytesMin, br.targetBytesMax)
	c.setQualityInternal(c.quality)
}

func (c *Codec) encodedLength() int {
	n := 3
	if c.dcShift > 0 {
		n = 5
	}
	for i := range c.slices {
		n += 4 + c.slices[i].dc.EncodedLength()
		n += 4 + c.slices[i].ac.EncodedLength()
	}
	return n
}

func saveTo(dst []byte, slices []sliceSet, format ImageFormat, quality, dcShift int) (int, error) {
	if len(dst) < 5 {
		return 0, ErrOutputTooSmall{Need: 5, Have: len(dst)}
	}
	sc := len(slices)
	scByte := byte(sc)
	if sc == 270 {
		scByte = 14
	}
	var b int
	if dcShift > 0 {
		dst[0] = codecFormatExtended
		dst[1] = byte(dcShift)
		dst[2] = codecFormatProgressive
		dst[3] = byte(quality)
		dst[4] = scByte
		b = 5
	} else {
		dst[0] = codecFormatProgressive
		dst[1] = byte(quality)
		dst[2] = scByte
		b = 3
	}
	for i := range slices {
		length := slices[i].dc.EncodedLength()
		if length > len(slices[i].dc.Stream) {
			return 0, ErrBitstream
		}
		if b+4+length > len(dst) {
			return 0, ErrOutputTooSmall{Need: b + 4 + length, Have: len(dst)}
		}
		binary.LittleEndian.PutUint32(dst[b:b+4], uint32(length))
		b += 4
		copy(dst[b:b+length], slices[i].dc.Stream[:length])
		b += length
	}
	for i := range slices {
		length := slices[i].ac.EncodedLength()
		if length > len(slices[i].ac.Stream) {
			return 0, ErrBitstream
		}
		if b+4+length > len(dst) {
			return 0, ErrOutputTooSmall{Need: b + 4 + length, Have: len(dst)}
		}
		binary.LittleEndian.PutUint32(dst[b:b+4], uint32(length))
		b += 4
		copy(dst[b:b+length], slices[i].ac.Stream[:length])
		b += length
	}
	return b, nil
}

// LoadFrom parses a VMX1 bitstream.
func (c *Codec) LoadFrom(data []byte) error {
	h, err := parseAndLoad(data, len(c.slices), c.slices)
	if err != nil {
		return err
	}
	c.setQualityInternal(h.quality)
	c.dcShift = h.dcShift
	return nil
}

func parseAndLoad(data []byte, expected int, slices []sliceSet) (frameHeader, error) {
	var h frameHeader
	if len(data) < 3 {
		return h, fmt.Errorf("%w: short", ErrBitstream)
	}
	offset := 0
	dcShift := 0
	b0 := data[0]
	if b0 != codecFormatProgressive && b0 != codecFormatInterlaced && b0 != codecFormatExtended {
		return h, ErrInvalidCodecFormat
	}
	if b0 == codecFormatExtended {
		if len(data) < 5 {
			return h, ErrBitstream
		}
		offset = 2
		dcShift = int(data[1])
	}
	quality := int(data[offset+1])
	sliceCount := int(data[offset+2])
	if sliceCount == 14 && expected == 270 {
		sliceCount = 270
	}
	if sliceCount != expected {
		return h, ErrInvalidSliceCount
	}
	h.quality = quality
	h.dcShift = dcShift
	h.sliceCount = sliceCount
	h.format = FormatUYVY
	b := offset + 3
	for i := 0; i < sliceCount; i++ {
		if b+4 > len(data) {
			return h, ErrBitstream
		}
		length := int(binary.LittleEndian.Uint32(data[b : b+4]))
		b += 4
		if b+length > len(data) || length > slices[i].dc.MaxLength {
			return h, ErrBitstream
		}
		copy(slices[i].dc.Stream[:length], data[b:b+length])
		slices[i].dc.StreamLength = length
		slices[i].dc.Pos = length
		b += length
	}
	if b < len(data) {
		for i := 0; i < sliceCount; i++ {
			if b+4 > len(data) {
				return h, ErrBitstream
			}
			length := int(binary.LittleEndian.Uint32(data[b : b+4]))
			b += 4
			if b+length > len(data) || length > slices[i].ac.MaxLength {
				return h, ErrBitstream
			}
			copy(slices[i].ac.Stream[:length], data[b:b+length])
			slices[i].ac.StreamLength = length
			slices[i].ac.Pos = length
			b += length
		}
	} else {
		for i := 0; i < sliceCount; i++ {
			slices[i].ac.StreamLength = 0
			slices[i].ac.Pos = 0
		}
	}
	return h, nil
}

// PreviewBitstreamLength returns the DC-only prefix length of data.
func PreviewBitstreamLength(data []byte) (int, error) {
	if len(data) < 3 {
		return 0, ErrBitstream
	}
	b0 := data[0]
	if b0 != codecFormatProgressive && b0 != codecFormatInterlaced && b0 != codecFormatExtended {
		return 0, ErrInvalidCodecFormat
	}
	offset := 0
	if b0 == codecFormatExtended {
		if len(data) < 5 {
			return 0, ErrBitstream
		}
		offset = 2
	}
	if data[offset] == codecFormatInterlaced {
		return 0, ErrUnsupported
	}
	if data[offset] != codecFormatProgressive {
		return 0, ErrInvalidCodecFormat
	}
	sliceCount := int(data[offset+2])
	if sliceCount == 14 {
		sliceCount = 270
	}
	if sliceCount == 0 {
		return 0, ErrInvalidSliceCount
	}
	b := offset + 3
	for i := 0; i < sliceCount; i++ {
		if b+4 > len(data) {
			return 0, ErrBitstream
		}
		length := int(binary.LittleEndian.Uint32(data[b : b+4]))
		b += 4
		if b+length > len(data) {
			return 0, ErrBitstream
		}
		b += length
	}
	return b, nil
}
