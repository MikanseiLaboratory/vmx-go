// Package bits implements the exp-Golomb slice bitstream used by VMX1.
package bits

import (
	"encoding/binary"
	"math/bits"

	"github.com/MikanseiLaboratory/vmx-go/internal/tables"
)

// SliceData is the per-slice DC or AC bitstream state.
type SliceData struct {
	Stream       []byte
	Pos          int
	MaxLength    int
	StreamLength int
	BitsLeft     int
	Temp         uint64
	TempRead     uint64
}

func New(capacity int) *SliceData {
	if capacity < 16 {
		capacity = 16
	}
	s := make([]byte, capacity)
	for i := range s {
		s[i] = 0xFF
	}
	return &SliceData{Stream: s, MaxLength: capacity, BitsLeft: 64, TempRead: binary.BigEndian.Uint64(s[:8])}
}

// ResetEncode prepares the writer.
func (s *SliceData) ResetEncode() {
	s.Pos = 0
	s.BitsLeft = 64
	s.Temp = 0
	s.StreamLength = 0
}

// ResetDecode prepares the reader for the current StreamLength.
func (s *SliceData) ResetDecode() {
	s.Pos = 0
	s.BitsLeft = 64
	s.Temp = 0
	s.TempRead = s.readU64BE(0)
}

func (s *SliceData) readU64BE(offset int) uint64 {
	limit := len(s.Stream)
	if s.StreamLength > 0 {
		limit = s.StreamLength
	}
	if offset < 0 || offset >= limit {
		return 0
	}
	if offset+8 <= limit && offset+8 <= len(s.Stream) {
		return binary.BigEndian.Uint64(s.Stream[offset : offset+8])
	}
	var buf [8]byte
	end := offset + 8
	if end > limit {
		end = limit
	}
	if end > len(s.Stream) {
		end = len(s.Stream)
	}
	copy(buf[:], s.Stream[offset:end])
	return binary.BigEndian.Uint64(buf[:])
}

func (s *SliceData) writeU64BE(offset int, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	end := offset + 8
	if end > len(s.Stream) {
		end = len(s.Stream)
	}
	n := end - offset
	if n > 0 {
		copy(s.Stream[offset:offset+n], buf[:n])
	}
}

func (s *SliceData) FlushReadBits() {
	if s.BitsLeft == 0 {
		s.BitsLeft = 64
		s.Pos += 8
		s.TempRead = s.readU64BE(s.Pos)
	}
}

func (s *SliceData) ReloadBits() {
	if s.BitsLeft < 32 {
		n := (64 - s.BitsLeft) >> 3
		s.Pos += n
		s.TempRead = s.readU64BE(s.Pos)
		s.BitsLeft += n << 3
	}
}

func (s *SliceData) GetBit() uint64 {
	s.BitsLeft--
	v := (s.TempRead >> uint(s.BitsLeft)) & 1
	s.FlushReadBits()
	return v
}

func (s *SliceData) GetBitB() uint64 {
	s.BitsLeft--
	return (s.TempRead >> uint(s.BitsLeft)) & 1
}

func (s *SliceData) GetBits(n uint32) uint64 {
	var out uint64
	for n > 0 {
		b := n
		if int(b) > s.BitsLeft {
			b = uint32(s.BitsLeft)
		}
		if out != 0 {
			out <<= b
		}
		s.BitsLeft -= int(b)
		out |= (s.TempRead >> uint(s.BitsLeft)) & mask64(b)
		n -= b
		s.FlushReadBits()
	}
	return out
}

func (s *SliceData) GetBitsB(n uint32) uint64 {
	s.BitsLeft -= int(n)
	return (s.TempRead >> uint(s.BitsLeft)) & mask64(n)
}

func (s *SliceData) GetZeros() uint64 {
	shifted := s.TempRead << uint((64-s.BitsLeft)&63)
	nz := uint64(bits.LeadingZeros64(shifted))
	if shifted == 0 {
		nz = 64
	}
	if int(nz) >= s.BitsLeft {
		nz = uint64(s.BitsLeft)
		s.BitsLeft = 0
		s.FlushReadBits()
		nz2 := uint64(bits.LeadingZeros64(s.TempRead))
		if s.TempRead == 0 {
			nz2 = 64
		}
		s.BitsLeft -= int(nz2)
		nz += nz2
	} else {
		s.BitsLeft -= int(nz)
	}
	return nz
}

func (s *SliceData) GetZerosB() uint64 {
	shifted := s.TempRead << uint((64-s.BitsLeft)&63)
	nz := uint64(bits.LeadingZeros64(shifted))
	if shifted == 0 {
		nz = 64
	}
	s.BitsLeft -= int(nz)
	return nz
}

func (s *SliceData) FlushRemainingReadBits() {
	if s.BitsLeft < 64 {
		r := uint32(s.BitsLeft & 7)
		s.GetBits(r)
	}
	s.FlushReadBits()
}

func (s *SliceData) RewindOverread(terms uint64) {
	if terms > 0 && terms < 64 {
		s.BitsLeft += int(tables.GolombZeroCodeLUT[terms].Length)
	}
}

func (s *SliceData) EmitBits32() {
	if s.BitsLeft < 33 {
		s.writeU64BE(s.Pos, s.Temp)
		s.Temp <<= 32
		s.Pos += 4
		s.BitsLeft += 32
	}
}

func (s *SliceData) FlushRemainingBits() {
	if s.BitsLeft < 64 {
		bitsToWrite := 64 - s.BitsLeft
		for bitsToWrite > 0 {
			if s.Pos < len(s.Stream) {
				s.Stream[s.Pos] = byte(s.Temp >> 56)
			}
			s.Pos++
			s.Temp <<= 8
			bitsToWrite -= 8
		}
		s.BitsLeft = 64
		s.Temp = 0
	}
}

func (s *SliceData) EncodeDC(val int16) {
	if val != 0 {
		input := uint32(Get2MagSign(val)) + 1
		s.BitsLeft -= int(tables.GolombCodeLength(input))
		s.Temp |= shl64(uint64(input), s.BitsLeft)
	} else {
		s.BitsLeft -= 2
		s.Temp |= shl64(3, s.BitsLeft)
	}
}

func (s *SliceData) EncodeZeros(numZeros *uint32) {
	if *numZeros != 0 {
		bc := uint32(32 - bits.LeadingZeros32(*numZeros))
		idx := s.BitsLeft
		if idx < 0 {
			idx = 0
		} else if idx > 64 {
			idx = 64
		}
		s.Temp |= tables.BitsLeftLookup[idx]
		s.BitsLeft -= int(bc + bc)
		s.Temp |= shl64(uint64(*numZeros), s.BitsLeft)
		*numZeros = 0
	}
}

func (s *SliceData) EncodeZerosSmall(nz uint64) {
	code := tables.GolombZeroCodeLUT[nz&63]
	s.BitsLeft -= int(code.Length)
	s.Temp |= shl64(code.Value, s.BitsLeft)
}

func (s *SliceData) EncodeValue(input uint32) {
	s.BitsLeft -= int(tables.GolombCodeLength(input))
	s.Temp |= shl64(uint64(input), s.BitsLeft)
}

func (s *SliceData) PeekGolombLookup() tables.GolombLookup {
	shift := s.BitsLeft - 12
	if shift < 0 {
		shift = 0
	}
	idx := int((s.TempRead >> uint(shift)) & 0xFFF)
	return tables.GolombLookupLUT[idx]
}

func (s *SliceData) EncodedLength() int { return s.Pos }

func shl64(v uint64, n int) uint64 { return v << (uint(n) & 63) }

func mask64(n uint32) uint64 {
	if n >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << n) - 1
}

func Get2MagSign(input int16) int16 {
	return int16((uint16(input) + uint16(input)) ^ uint16(input>>15))
}

func GolombSymbol(coeff int16) uint32 {
	return uint32(Get2MagSign(coeff)) + 1
}

func GetIntFrom2MagSign(i uint64) int16 {
	v := int32(uint32(i))
	t := (v + (v & 1)) >> 1
	s := (v + (v & 1)) * (v & 1)
	return int16(t - s)
}
