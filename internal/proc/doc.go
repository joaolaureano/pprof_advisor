// Package proc holds the process handling shared by the packages that shell out
// to the Go toolchain.
//
// It exists because `go test` and `go build` both spawn children of their own —
// a test binary, a compiler — and killing only the process we started leaves
// those running. The platform-specific way to reap the whole group belongs in
// one place rather than in every package that runs a subprocess.
package proc
