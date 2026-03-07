//go:build !llgo
// +build !llgo

package plan9asm

import "testing"

func TestTranslateAMD64PEXTRDToMemory(t *testing.T) {
	src := `
TEXT pextrdmem(SB),NOSPLIT,$0-0
	PEXTRD $3, X5, 16(DI)
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"pextrdmem": {Name: "pextrdmem", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateAMD64CallThroughMemory(t *testing.T) {
	src := `
TEXT callmem(SB),NOSPLIT,$0-0
	CALL (BX)(CX*8)
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"callmem": {Name: "callmem", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranslateAMD64MovDAliasToGPR(t *testing.T) {
	src := `
TEXT movdalias(SB),NOSPLIT,$0-0
	MOVD $0, BP
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "x86_64-unknown-linux-gnu",
		Sigs: map[string]FuncSig{
			"movdalias": {Name: "movdalias", Ret: Void},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
}
