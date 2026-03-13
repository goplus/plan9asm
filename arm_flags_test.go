package plan9asm

import (
	"strings"
	"testing"
)

func TestTranslateARMAddSFeedsConditional(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·f(SB),NOSPLIT,$0-0
	MOVW $0, R0
	ADD.S $0, R0
	MOVW.EQ $1, R1
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	ll, err := Translate(file, Options{
		TargetTriple: "armv7-unknown-linux-gnueabihf",
		Goarch:       "arm",
		ResolveSym:   func(sym string) string { return "example." + strings.TrimPrefix(sym, "·") },
		Sigs: map[string]FuncSig{
			"example.f": {Name: "example.f", Ret: Void},
		},
	})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if !strings.Contains(ll, "%flags_z") {
		t.Fatalf("missing flag writes in output:\n%s", ll)
	}
}

func TestTranslateARMAdditionalConditions(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·f(SB),NOSPLIT,$0-0
	CMP R0, R0
	MOVW.PL $1, R1
	MOVW.VS $2, R2
	MOVW.VC $3, R3
	MOVW.AL $4, R4
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "armv7-unknown-linux-gnueabihf",
		Goarch:       "arm",
		ResolveSym:   func(sym string) string { return "example." + strings.TrimPrefix(sym, "·") },
		Sigs: map[string]FuncSig{
			"example.f": {Name: "example.f", Ret: Void},
		},
	})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
}

func TestTranslateARMRejectsUnresolvedSymbolicImmediate(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·f(SB),NOSPLIT,$0-0
	MOVW $(16 + callbackArgs__size), R0
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, err = Translate(file, Options{
		TargetTriple: "armv7-unknown-linux-gnueabihf",
		Goarch:       "arm",
		ResolveSym:   func(sym string) string { return "example." + strings.TrimPrefix(sym, "·") },
		Sigs: map[string]FuncSig{
			"example.f": {Name: "example.f", Ret: Void},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unresolved symbolic immediate") {
		t.Fatalf("Translate() error = %v, want unresolved symbolic immediate", err)
	}
}
