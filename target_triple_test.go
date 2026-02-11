//go:build !llgo
// +build !llgo

package plan9asm

// testTargetTriple returns a practical LLVM target triple for tests.
// We only need a small set for current coverage.
func testTargetTriple(goos, goarch string) string {
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-macosx"
		case "arm64":
			return "arm64-apple-macosx"
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu"
		case "arm64":
			return "aarch64-unknown-linux-gnu"
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc"
		case "arm64":
			return "aarch64-pc-windows-msvc"
		}
	}

	// Fallback for unsupported combos in tests.
	return goarch + "-unknown-" + goos
}
