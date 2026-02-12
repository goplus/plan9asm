package plan9asm

import "strings"

func emitArchPrelude(b *strings.Builder, arch Arch, goarch string) {
	switch arch {
	case ArchARM64:
		emitARM64Prelude(b)
	case ArchAMD64:
		// Keep historical gating to avoid injecting x86-only prelude when
		// parsing amd64 syntax for non-amd64 targets in tests.
		if goarch == "amd64" {
			emitAMD64Prelude(b)
		}
	}
}
