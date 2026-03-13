package plan9asm

import "testing"

func TestParseARMRegRangeList(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT runtime·memclrNoHeapPointers(SB),NOSPLIT,$0-8
	MOVM.IA.W [R0-R7], (R8)
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	got := file.Funcs[0].Instrs[1].Args[0]
	if got.Kind != OpRegList || len(got.RegList) != 8 || got.RegList[0] != Reg("R0") || got.RegList[7] != Reg("R7") {
		t.Fatalf("unexpected reg list: %#v", got)
	}
}

func TestParseARMShiftOperands(t *testing.T) {
	file, err := Parse(ArchARM, `TEXT ·block(SB),NOSPLIT,$0-0
	ADD	R2@>(32-7), R3, R2
	MOVW	R4>>R5, R6
	TEQ	R7->1, R7
	RET
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	ins := file.Funcs[0].Instrs
	if ins[1].Args[0].Kind != OpRegShift || ins[1].Args[0].ShiftOp != ShiftRotate || ins[1].Args[0].ShiftAmount != 25 {
		t.Fatalf("unexpected rotate operand: %#v", ins[1].Args[0])
	}
	if ins[2].Args[0].Kind != OpRegShift || ins[2].Args[0].ShiftOp != ShiftRight || ins[2].Args[0].ShiftReg != Reg("R5") {
		t.Fatalf("unexpected register shift operand: %#v", ins[2].Args[0])
	}
	if ins[3].Args[0].Kind != OpRegShift || ins[3].Args[0].ShiftOp != ShiftArith || ins[3].Args[0].ShiftAmount != 1 {
		t.Fatalf("unexpected arithmetic shift operand: %#v", ins[3].Args[0])
	}
}
