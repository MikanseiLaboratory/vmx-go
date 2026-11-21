# vmx-go

Pure Go VMX1 video codec — no cgo and no libvmx linkage.

[![CI](https://github.com/MikanseiLaboratory/vmx-go/actions/workflows/ci.yml/badge.svg)](https://github.com/MikanseiLaboratory/vmx-go/actions/workflows/ci.yml)

## Status

The scalar path is byte-identical to `vmx-rs` for the tested 8-bit progressive
4:2:2 fixtures, and decodes Rust-produced VMX1 bitstreams exactly. This is the
codec used by the `omt-go` examples.

Implemented:

- VMX1 8-bit progressive 4:2:2 encode/decode
- UYVY and YUY2 input; UYVY, YUY2, RGBA and BGRA output
- Slice-parallel encode/decode
- SIMD UYVY→planar color kernel (Go 1.27 `simd/archsimd`, NEON/AVX2)
- Rust-compatible bitstream container and quality/profile selection

Not yet implemented:

- Alpha plane encode/decode
- 1/8 preview decode
- 10/16-bit and interlaced bitstreams
- GPU decode / texture output
- VMXF container helpers

## Usage

```go
codec, err := vmx.New(vmx.Config{
    Width: 1920, Height: 1080,
    Profile: vmx.ProfileOmtSQ,
    ColorSpace: vmx.ColorBT709,
})
if err != nil { return err }

if err := codec.EncodeUYVY(frame, 1920*2); err != nil { return err }
packet := make([]byte, codec.MaxEncodedSize())
n, err := codec.SaveTo(packet)
if err != nil { return err }

dec, err := vmx.New(vmx.Config{Width: 1920, Height: 1080, ColorSpace: vmx.ColorBT709})
if err != nil { return err }
if err := dec.LoadFrom(packet[:n]); err != nil { return err }
rgba := make([]byte, 1920*1080*4)
if err := dec.DecodeRGBA(rgba, 1920*4); err != nil { return err }
```

## Go version and SIMD

The package requires Go 1.27+. The SIMD color kernel is behind the experimental
SIMD package:

```sh
GOEXPERIMENT=simd go test ./...
```

Without `GOEXPERIMENT=simd`, `vmx-go` builds and runs with the scalar path.
`Codec.SimdPath()` reports the DCT path (`scalar` for v0.1) and
`Codec.ColorSimdPath()` reports the color path (`neon` / `avx2` / `scalar`).

## Performance

On an Apple M3 (8 cores), 1920×1080:

- encode: ~20 ms/frame
- decode + RGBA conversion: ~12 ms/frame

720p decode is ~7 ms/frame. The DCT/IDCT is currently scalar; slice
parallelism and the SIMD color kernel are the only optimizations in v0.1.

## Verification

`go test ./...` includes:

- byte-for-byte comparison against Rust-generated bitstreams (`testdata/`)
- decode comparison against the Rust decoder output
- SIMD/scalar plane equality
- roundtrip and boundary tests
- adaptive-quality sequence parity with libvmx/vmx-rs (float32 bitrate targets)

Validated on Windows 11 amd64 with Go 1.27, both scalar and
`GOEXPERIMENT=simd`; the encoder/decoder also match the scalar Rust
implementation for odd widths, non-multiple-of-16 heights, and all profiles.

## License

MIT. VMX1 tables and format details are derived from the MIT-licensed
[libvmx](https://github.com/openmediatransport/libvmx) and
[vmx-rs](https://github.com/MikanseiLaboratory/vmx-rs). See [NOTICE](NOTICE).
