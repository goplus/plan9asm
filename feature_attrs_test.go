//go:build !llgo
// +build !llgo

package plan9asm

import "testing"

func TestInferFuncTargetFeatures(t *testing.T) {
	tests := []struct {
		name string
		arch Arch
		ops  []Op
		want string
	}{
		{
			name: "amd64 crc32",
			arch: ArchAMD64,
			ops:  []Op{"CRC32B", "CRC32Q"},
			want: "+crc32,+sse4.2",
		},
		{
			name: "amd64 pclmul",
			arch: ArchAMD64,
			ops:  []Op{"PCLMULQDQ"},
			want: "+pclmul,+sse4.1",
		},
		{
			name: "amd64 pshufb",
			arch: ArchAMD64,
			ops:  []Op{"PSHUFB", "VPSHUFB"},
			want: "+ssse3",
		},
		{
			name: "amd64 aes",
			arch: ArchAMD64,
			ops:  []Op{"AESENC", "AESENCLAST", "AESDEC", "AESDECLAST", "AESIMC", "AESKEYGENASSIST"},
			want: "+aes",
		},
		{
			name: "amd64 combined sorted deduped",
			arch: ArchAMD64,
			ops:  []Op{"AESENC", "CRC32L", "PCLMULQDQ", "PSHUFB", "CRC32Q", "AESDEC", "VPSHUFB"},
			want: "+aes,+crc32,+pclmul,+sse4.1,+sse4.2,+ssse3",
		},
		{
			name: "arm64 crc32",
			arch: ArchARM64,
			ops:  []Op{"CRC32CX", "CRC32W"},
			want: "+crc",
		},
		{
			name: "no feature attrs",
			arch: ArchAMD64,
			ops:  []Op{"MOVQ", "RET"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := Func{Instrs: make([]Instr, len(tt.ops))}
			for i, op := range tt.ops {
				fn.Instrs[i] = Instr{Op: op}
			}
			if got := inferFuncTargetFeatures(tt.arch, fn); got != tt.want {
				t.Fatalf("inferFuncTargetFeatures(%q, %v) = %q, want %q", tt.arch, tt.ops, got, tt.want)
			}
		})
	}
}
