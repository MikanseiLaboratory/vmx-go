//go:build !goexperiment.simd || (!amd64 && !arm64)

package simd

// UYVYToPlanar is unavailable without the experimental SIMD package.
func UYVYToPlanar(src []byte, stride int, y, u, v []byte, ys, us, vs, width, height int) bool {
	return false
}

// ColorPath reports the selected color kernel.
func ColorPath() string { return "scalar" }
