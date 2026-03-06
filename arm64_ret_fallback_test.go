package plan9asm

import (
	"strings"
	"testing"
)

func translateARM64WithSig(t *testing.T, src string, sig FuncSig, resolvedName string) string {
	t.Helper()
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatalf("parse asm: %v", err)
	}
	resolve := func(sym string) string {
		sym = strings.TrimSuffix(sym, "<ABIInternal>")
		sym = strings.TrimSuffix(sym, "<>")
		if strings.HasPrefix(sym, "·") {
			return resolvedName
		}
		return sym
	}
	sig.Name = resolvedName
	ll, err := Translate(file, Options{
		ResolveSym: resolve,
		Sigs:       map[string]FuncSig{resolvedName: sig},
		Goarch:     "arm64",
	})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	return ll
}

func TestARM64ReturnFallbackToRegisterWhenResultSlotNotWritten(t *testing.T) {
	src := `
TEXT ·RetArg<ABIInternal>(SB),NOSPLIT,$0-16
	RET
`
	sig := FuncSig{
		Args: []LLVMType{I64},
		Ret:  I64,
		Frame: FrameLayout{
			Results: []FrameSlot{{Offset: 8, Type: I64, Index: 0, Field: -1}},
		},
	}
	ll := translateARM64WithSig(t, src, sig, "test.RetArg")
	if strings.Contains(ll, "ret i64 0") {
		t.Fatalf("unexpected zero return when result slot is not written:\n%s", ll)
	}
	if !strings.Contains(ll, "ret i64 %") {
		t.Fatalf("expected register-based non-constant return:\n%s", ll)
	}
}
