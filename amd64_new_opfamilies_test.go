//go:build !llgo
// +build !llgo

package plan9asm

import "testing"

func TestTranslateAMD64SHA1Family(t *testing.T) {
	src := `
TEXT sha1ops(SB),NOSPLIT,$0-0
	SHA1MSG1 X2, X1
	SHA1MSG2 X3, X1
	SHA1NEXTE X4, X1
	SHA1RNDS4 $3, X5, X1
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"sha1ops": {Name: "sha1ops", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateAMD64AVX512CRCSubset(t *testing.T) {
	src := `
GLOBL table<>(SB), RODATA, $64
DATA table<>+0(SB)/8, $0

TEXT crcops(SB),NOSPLIT,$0-0
	VPXORQ Z10, Z10, Z10
	VMOVAPS X0, X10
	VMOVDQU64 (AX), Z1
	VPXORQ Z10, Z1, Z1
	VMOVDQU64 table<>+0(SB), Z0
	VPCLMULQDQ $0x11, Z0, Z1, Z5
	VPTERNLOGD $0x96, Z11, Z5, Z1
	VEXTRACTF32X4 $1, Z1, X2
	VZEROUPPER
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"crcops": {Name: "crcops", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateAMD64AVX512Go126Families(t *testing.T) {
	src := `
GLOBL mat<>(SB), RODATA, $64
DATA mat<>+0(SB)/8, $0

TEXT go126ops(SB),NOSPLIT,$0-0
	MOVQ $0xff, AX
	KMOVQ AX, K1
	VPERMB Z4, Z0, Z0
	VGF2P8AFFINEQB $0, mat<>+0(SB), Z0, Z0
	VPERMI2B Z3, Z2, Z1
	VPERMI2B.Z Z3, Z2, K1, Z5
	VPERMB.Z Z4, Z6, K1, Z0
	VPOPCNTB Z1, Z3
	VPCMPUQ $4, Z1, Z15, K1
	VPCOMPRESSQ Z1, K1, Z2
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"go126ops": {Name: "go126ops", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}
