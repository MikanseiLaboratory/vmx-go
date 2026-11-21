# vmx-go

Pure Go VMX1 video codec. No cgo, no libvmx linkage.

[![CI](https://github.com/MikanseiLaboratory/vmx-go/actions/workflows/ci.yml/badge.svg)](https://github.com/MikanseiLaboratory/vmx-go/actions/workflows/ci.yml)

## Features

- VMX1 8-bit progressive 4:2:2 encode/decode
- UYVY and YUY2 input; UYVY, YUY2, RGBA and BGRA output
- Slice-parallel encode/decode
- SIMD UYVY→planar color kernel (AVX2 on amd64, NEON on arm64)
- Bitstream-compatible with libvmx and vmx-rs

Not implemented: alpha, 1/8 preview, 10/16-bit, interlaced, GPU decode,
VMXF containers.

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

## Requirements

Go 1.27 or newer.

SIMD is optional:

```sh
GOEXPERIMENT=simd go test ./...
```

Without it the scalar path is used. `Codec.ColorSimdPath()` reports the
active color kernel (`avx2`, `neon`, or `scalar`).

## Test

```sh
go test ./...
go vet ./...
```

## License

MIT. VMX1 tables and format details are derived from the MIT-licensed
[libvmx](https://github.com/openmediatransport/libvmx) and
[vmx-rs](https://github.com/MikanseiLaboratory/vmx-rs). See [NOTICE](NOTICE).
