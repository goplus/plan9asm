package plan9asm

import "testing"

func TestParseARMSymbolicImmediateExpr(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·f(SB),NOSPLIT,$0-0
	MOVW $(16 + callbackArgs__size), R0
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := file.Funcs[0].Instrs[1].Args[0]; got.Kind != OpImm {
		t.Fatalf("unexpected operand kind: %#v", got)
	}
}
