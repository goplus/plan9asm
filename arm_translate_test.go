package plan9asm

import (
	"strings"
	"testing"
)

func TestTranslateARMLinearAdd(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·Add(SB),NOSPLIT,$0-12
	MOVW	a+0(FP), R0
	ADD	b+4(FP), R0
	MOVW	R0, ret+8(FP)
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
			"example.Add": {
				Name: "example.Add",
				Args: []LLVMType{I32, I32},
				Ret:  I32,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I32, Index: 0, Field: -1},
						{Offset: 4, Type: I32, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 8, Type: I32, Index: 0, Field: -1},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	for _, want := range []string{
		"target triple = \"armv7-unknown-linux-gnueabihf\"",
		"define i32 @example.Add(i32 %arg0, i32 %arg1)",
		"add i32",
		"ret i32",
	} {
		if !strings.Contains(ll, want) {
			t.Fatalf("missing %q in output:\n%s", want, ll)
		}
	}
}
