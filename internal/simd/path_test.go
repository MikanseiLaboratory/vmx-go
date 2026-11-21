//go:build goexperiment.simd && (amd64 || arm64)

package simd

import "testing"

// TestColorPathMatchesKernel makes sure ColorSimdPath never claims a SIMD path
// when UYVYToPlanar will silently fall back to the scalar converter.
func TestColorPathMatchesKernel(t *testing.T) {
	const w, h = 8, 1
	src := make([]byte, w*2*h)
	y := make([]byte, w*h)
	u := make([]byte, w/2*h)
	v := make([]byte, w/2*h)
	ok := UYVYToPlanar(src, w*2, y, u, v, w, w/2, w/2, w, h)
	path := ColorPath()
	if ok && path == "scalar" {
		t.Fatalf("kernel reported unavailable but path is %q", path)
	}
	if !ok && path != "scalar" {
		t.Fatalf("kernel returned false but path reports %q", path)
	}
}
