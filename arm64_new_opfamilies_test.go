//go:build !llgo
// +build !llgo

package plan9asm

import "testing"

func TestTranslateARM64SHA3Families(t *testing.T) {
	src := `
TEXT sha3ops(SB),NOSPLIT,$0-0
	VEOR3	V20.B16, V15.B16, V10.B16, V25.B16
	VRAX1	V27.D2, V25.D2, V30.D2
	VXAR	$63, V30.D2, V1.D2, V25.D2
	VBCAX	V8.B16, V22.B16, V26.B16, V20.B16
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "aarch64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"sha3ops": {Name: "sha3ops", Ret: Void},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateARM64BarrierFamilies(t *testing.T) {
	src := `
TEXT barrierops(SB),NOSPLIT,$0-0
	DSB $7
	ISB $15
	DC ZVA, R0
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "aarch64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"barrierops": {Name: "barrierops", Ret: Void},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateARM64FeatureProbeMRS(t *testing.T) {
	src := `
TEXT featureprobe(SB),NOSPLIT,$0-0
	MRS ID_AA64ISAR0_EL1, R0
	MRS ID_AA64PFR0_EL1, R1
	MRS ID_AA64ZFR0_EL1, R2
	MRS MIDR_EL1, R3
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "aarch64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"featureprobe": {Name: "featureprobe", Ret: Void},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateARM64CompareWithExtendedRegister(t *testing.T) {
	src := `
TEXT countcmp(SB),NOSPLIT,$0-0
	CMP	R2.UXTB, R5
	CINC	EQ, R11, R11
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "aarch64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"countcmp": {Name: "countcmp", Ret: Void},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}
}
