package plan9asm

import (
	"strings"
	"testing"
)

func newAMD64CtxWithFuncForTest(t *testing.T, fn Func, sig FuncSig, sigs map[string]FuncSig) (*amd64Ctx, *strings.Builder) {
	t.Helper()
	if sig.Name == "" {
		sig.Name = "example.f"
	}
	if sigs == nil {
		sigs = map[string]FuncSig{}
	}
	var b strings.Builder
	c := newAMD64Ctx(&b, fn, sig, func(sym string) string {
		sym = goStripABISuffix(sym)
		sym = strings.ReplaceAll(sym, "∕", "/")
		if strings.HasPrefix(sym, "runtime·") {
			return strings.ReplaceAll(sym, "·", ".")
		}
		if strings.HasPrefix(sym, "·") {
			return "example." + strings.TrimPrefix(sym, "·")
		}
		if !strings.Contains(sym, "·") && !strings.Contains(sym, ".") && !strings.Contains(sym, "/") {
			return "example." + sym
		}
		return strings.ReplaceAll(sym, "·", ".")
	}, sigs, false)
	if err := c.emitEntryAllocas(); err != nil {
		t.Fatalf("emitEntryAllocas() error = %v", err)
	}
	return c, &b
}

func TestAMD64CtxHelperEdges(t *testing.T) {
	fn := Func{
		Instrs: []Instr{
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("X0")}, {Kind: OpReg, Reg: Reg("X1")}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("Y2")}, {Kind: OpReg, Reg: Reg("Y3")}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("Z4")}, {Kind: OpReg, Reg: Reg("Z5")}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("K1")}, {Kind: OpReg, Reg: Reg("K2")}}},
		},
	}
	sig := FuncSig{
		Name: "example.ctx",
		Args: []LLVMType{Ptr, I1, I8, I16, I32, I64, LLVMType("double"), LLVMType("float")},
		Ret:  LLVMType("{i32, ptr, double, float, i16}"),
		Frame: FrameLayout{
			Params: []FrameSlot{
				{Offset: 0, Type: Ptr, Index: 0, Field: -1},
				{Offset: 8, Type: I1, Index: 1, Field: -1},
				{Offset: 16, Type: I8, Index: 2, Field: -1},
				{Offset: 24, Type: I16, Index: 3, Field: -1},
				{Offset: 32, Type: I32, Index: 4, Field: -1},
				{Offset: 40, Type: I64, Index: 5, Field: -1},
				{Offset: 48, Type: LLVMType("double"), Index: 6, Field: -1},
				{Offset: 56, Type: LLVMType("float"), Index: 7, Field: -1},
			},
			Results: []FrameSlot{
				{Offset: 80, Type: I32, Index: 0, Field: -1},
				{Offset: 88, Type: Ptr, Index: 1, Field: -1},
				{Offset: 96, Type: LLVMType("double"), Index: 2, Field: -1},
				{Offset: 104, Type: LLVMType("float"), Index: 3, Field: -1},
				{Offset: 112, Type: I16, Index: 4, Field: -1},
			},
		},
	}
	c, b := newAMD64CtxWithFuncForTest(t, fn, sig, nil)

	for _, ty := range []LLVMType{Ptr, I1, I8, I16, I32, I64} {
		if got, ok, err := amd64ValueAsI64(c, ty, "%v"); err != nil || !ok || got == "" {
			t.Fatalf("amd64ValueAsI64(%s) = (%q, %v, %v)", ty, got, ok, err)
		}
	}
	if got, ok, err := amd64ValueAsI64(c, LLVMType("v2i64"), "%v"); err != nil || ok || got != "" {
		t.Fatalf("amd64ValueAsI64(unsupported) = (%q, %v, %v)", got, ok, err)
	}

	c.pushI64("7")
	if got := c.popI64(); got == "" {
		t.Fatalf("popI64() returned empty value")
	}

	if got, err := c.loadX("X0"); err != nil || got == "" {
		t.Fatalf("loadX(X0) = (%q, %v)", got, err)
	}
	if err := c.storeX("X1", "<16 x i8> zeroinitializer"); err != nil {
		t.Fatalf("storeX(X1) error = %v", err)
	}
	if _, err := c.loadX("AX"); err == nil {
		t.Fatalf("loadX(AX) unexpectedly succeeded")
	}
	if err := c.storeX("AX", "<16 x i8> zeroinitializer"); err == nil {
		t.Fatalf("storeX(AX) unexpectedly succeeded")
	}

	if got, err := c.loadY("Y2"); err != nil || got == "" {
		t.Fatalf("loadY(Y2) = (%q, %v)", got, err)
	}
	if err := c.storeY("Y3", "<32 x i8> zeroinitializer"); err != nil {
		t.Fatalf("storeY(Y3) error = %v", err)
	}
	if _, err := c.loadY("AX"); err == nil {
		t.Fatalf("loadY(AX) unexpectedly succeeded")
	}
	if err := c.storeY("AX", "<32 x i8> zeroinitializer"); err == nil {
		t.Fatalf("storeY(AX) unexpectedly succeeded")
	}

	if got, err := c.loadZ("Z4"); err != nil || got == "" {
		t.Fatalf("loadZ(Z4) = (%q, %v)", got, err)
	}
	if err := c.storeZ("Z5", "<64 x i8> zeroinitializer"); err != nil {
		t.Fatalf("storeZ(Z5) error = %v", err)
	}
	if _, err := c.loadZ("AX"); err == nil {
		t.Fatalf("loadZ(AX) unexpectedly succeeded")
	}
	if err := c.storeZ("AX", "<64 x i8> zeroinitializer"); err == nil {
		t.Fatalf("storeZ(AX) unexpectedly succeeded")
	}

	if got, err := c.loadK("K1"); err != nil || got == "" {
		t.Fatalf("loadK(K1) = (%q, %v)", got, err)
	}
	if err := c.storeK("K2", "9"); err != nil {
		t.Fatalf("storeK(K2) error = %v", err)
	}
	if _, err := c.loadK("AX"); err == nil {
		t.Fatalf("loadK(AX) unexpectedly succeeded")
	}
	if err := c.storeK("AX", "9"); err == nil {
		t.Fatalf("storeK(AX) unexpectedly succeeded")
	}

	c.setZFlagFromI64("1")
	c.setZSFlagsFromI64("2")
	c.setZSFlagsFromI32("3")
	c.setCmpFlags("4", "5")
	if got := c.loadFlag(c.flagsZSlot); got == "" {
		t.Fatalf("loadFlag() returned empty value")
	}

	if slot, ok := c.fpParam(32); !ok || slot.Type != I32 {
		t.Fatalf("fpParam(32) = (%#v, %v)", slot, ok)
	}
	if _, ok := c.fpParam(999); ok {
		t.Fatalf("fpParam(999) unexpectedly succeeded")
	}
	if alloca, ty, ok := c.fpResultAlloca(80); !ok || alloca == "" || ty != I32 {
		t.Fatalf("fpResultAlloca(80) = (%q, %q, %v)", alloca, ty, ok)
	}
	if _, _, ok := c.fpResultAlloca(999); ok {
		t.Fatalf("fpResultAlloca(999) unexpectedly succeeded")
	}
	c.markFPResultAddrTaken(88)
	c.markFPResultWritten(80)

	for _, off := range []int64{0, 8, 16, 24, 32, 40, 48, 56} {
		if got, err := c.evalFPToI64(off); err != nil || got == "" {
			t.Fatalf("evalFPToI64(%d) = (%q, %v)", off, got, err)
		}
	}
	c.fpParams[64] = FrameSlot{Offset: 64, Type: LLVMType("v4i32"), Index: 0}
	if _, err := c.evalFPToI64(64); err == nil {
		t.Fatalf("evalFPToI64(unsupported type) unexpectedly succeeded")
	}
	c.fpParams[72] = FrameSlot{Offset: 72, Type: I64, Index: 99}
	if _, err := c.evalFPToI64(72); err == nil {
		t.Fatalf("evalFPToI64(invalid index) unexpectedly succeeded")
	}

	if err := c.storeFPResult(80, I64, "11"); err != nil {
		t.Fatalf("storeFPResult(i64->i32) error = %v", err)
	}
	if err := c.storeFPResult(88, I64, "12"); err != nil {
		t.Fatalf("storeFPResult(i64->ptr) error = %v", err)
	}
	if err := c.storeFPResult(96, I64, "13"); err != nil {
		t.Fatalf("storeFPResult(i64->double) error = %v", err)
	}
	if err := c.storeFPResult(104, LLVMType("float"), "%f"); err != nil {
		t.Fatalf("storeFPResult(float->float) error = %v", err)
	}
	if err := c.storeFPResult(112, I8, "15"); err != nil {
		t.Fatalf("storeFPResult(i8->i16) error = %v", err)
	}
	if err := c.storeFPResult(96, LLVMType("double"), "%x"); err != nil {
		t.Fatalf("storeFPResult(double->double) error = %v", err)
	}
	if err := c.storeFPResult(80, Ptr, "%x"); err == nil {
		t.Fatalf("storeFPResult(ptr->i32) unexpectedly succeeded")
	}

	if got, err := c.loadFPResult(FrameSlot{Index: 0, Type: I32}); err != nil || got == "" {
		t.Fatalf("loadFPResult() = (%q, %v)", got, err)
	}
	if _, err := c.loadFPResult(FrameSlot{Index: 99, Type: I32}); err == nil {
		t.Fatalf("loadFPResult(missing) unexpectedly succeeded")
	}

	if !isAMD64FloatRetTy(LLVMType("double")) || !isAMD64FloatRetTy(LLVMType("float")) || isAMD64FloatRetTy(I64) {
		t.Fatalf("isAMD64FloatRetTy() mismatch")
	}
	if got, ok := c.retIntRegByOrd(1); !ok || got != BX {
		t.Fatalf("retIntRegByOrd(1) = (%q, %v)", got, ok)
	}
	if _, ok := c.retIntRegByOrd(-1); ok {
		t.Fatalf("retIntRegByOrd(-1) unexpectedly succeeded")
	}

	if err := c.storeReg(AX, "21"); err != nil {
		t.Fatalf("storeReg(AX) error = %v", err)
	}
	if err := c.storeReg(BX, "22"); err != nil {
		t.Fatalf("storeReg(BX) error = %v", err)
	}
	if err := c.storeX("X0", "<16 x i8> zeroinitializer"); err != nil {
		t.Fatalf("storeX(X0) error = %v", err)
	}
	for _, tc := range []struct {
		ord int
		ty  LLVMType
	}{
		{0, I64},
		{1, I32},
		{0, I16},
		{0, I8},
		{0, I1},
		{0, Ptr},
		{0, LLVMType("double")},
		{0, LLVMType("float")},
	} {
		if got, err := c.loadRetIntRegTyped(tc.ord, tc.ty); err != nil || got == "" {
			t.Fatalf("loadRetIntRegTyped(%d, %s) = (%q, %v)", tc.ord, tc.ty, got, err)
		}
	}
	if got, err := c.loadRetIntRegTyped(99, I64); err != nil || got != "0" {
		t.Fatalf("loadRetIntRegTyped(oob) = (%q, %v)", got, err)
	}
	if _, err := c.loadRetIntRegTyped(0, LLVMType("v2i64")); err == nil {
		t.Fatalf("loadRetIntRegTyped(unsupported) unexpectedly succeeded")
	}

	if got, err := c.loadRetFloatRegTyped(0, LLVMType("double")); err != nil || got == "" {
		t.Fatalf("loadRetFloatRegTyped(double) = (%q, %v)", got, err)
	}
	if got, err := c.loadRetFloatRegTyped(0, LLVMType("float")); err != nil || got == "" {
		t.Fatalf("loadRetFloatRegTyped(float) = (%q, %v)", got, err)
	}
	if got, err := c.loadRetFloatRegTyped(99, LLVMType("double")); err != nil || got != "0.000000e+00" {
		t.Fatalf("loadRetFloatRegTyped(oob) = (%q, %v)", got, err)
	}
	if _, err := c.loadRetFloatRegTyped(0, I64); err == nil {
		t.Fatalf("loadRetFloatRegTyped(unsupported) unexpectedly succeeded")
	}

	if isFloat, ord := c.retClassOrdinal(FrameSlot{Index: 3, Type: LLVMType("float")}); !isFloat || ord != 1 {
		t.Fatalf("retClassOrdinal(float) = (%v, %d)", isFloat, ord)
	}
	if got, err := c.loadRetSlotFallback(FrameSlot{Index: 0, Type: I32}); err != nil || got == "" {
		t.Fatalf("loadRetSlotFallback(int) = (%q, %v)", got, err)
	}
	if got, err := c.loadRetSlotFallback(FrameSlot{Index: 2, Type: LLVMType("double")}); err != nil || got == "" {
		t.Fatalf("loadRetSlotFallback(float) = (%q, %v)", got, err)
	}

	for _, tc := range []struct {
		in      string
		wantOK  bool
		wantOff int64
	}{
		{in: "foo<>(SB)", wantOK: true},
		{in: "foo+8(SB)", wantOK: true, wantOff: 8},
		{in: "bare_symbol", wantOK: true},
		{in: "4(AX)", wantOK: false},
		{in: "", wantOK: false},
	} {
		_, off, ok := parseSBRef(tc.in)
		if ok != tc.wantOK || off != tc.wantOff {
			t.Fatalf("parseSBRef(%q) = (%d, %v)", tc.in, off, ok)
		}
	}

	out := b.String()
	for _, want := range []string{
		"alloca <16 x i8>",
		"alloca <32 x i8>",
		"alloca <64 x i8>",
		"alloca i64",
		"ptrtoint ptr %arg0 to i64",
		"bitcast double %arg6 to i64",
		"bitcast float %arg7 to i32",
		"trunc i64 11 to i32",
		"inttoptr i64 12 to ptr",
		"bitcast i64 13 to double",
		"store float %f",
		"zext i8 15 to i16",
		"extractelement <2 x i64>",
		"extractelement <4 x i32>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestAMD64AtomicAndBranchEdges(t *testing.T) {
	fn := Func{
		Instrs: []Instr{
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: DI}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: SI}, {Kind: OpReg, Reg: DX}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("R8")}, {Kind: OpReg, Reg: Reg("R9")}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("X0")}, {Kind: OpReg, Reg: Reg("X1")}}},
		},
	}
	sigs := map[string]FuncSig{
		"example.helper": {
			Name:    "example.helper",
			Args:    []LLVMType{I64, I1, I8, I16, I32, Ptr},
			ArgRegs: []Reg{DI, SI, DX, CX, Reg("R8"), Reg("R9")},
			Ret:     Ptr,
		},
		"example.cast": {
			Name: "example.cast",
			Args: []LLVMType{I32, I64},
			Ret:  I64,
		},
		"example.voidret": {
			Name: "example.voidret",
			Args: []LLVMType{I64},
			Ret:  Void,
		},
		"example.badarg": {
			Name: "example.badarg",
			Args: []LLVMType{LLVMType("v2i64")},
			Ret:  I64,
		},
		"example.badret": {
			Name: "example.badret",
			Args: []LLVMType{I64},
			Ret:  LLVMType("double"),
		},
	}
	sig := FuncSig{
		Name: "example.caller",
		Args: []LLVMType{I64, Ptr},
		Ret:  I64,
		Frame: FrameLayout{
			Results: []FrameSlot{{Offset: 8, Type: I64, Index: 0}},
		},
	}
	c, b := newAMD64CtxWithFuncForTest(t, fn, sig, sigs)
	c.blocks = []amd64Block{{name: "entry"}, {name: "fall"}, {name: "target"}}
	c.blockBase = []int{0, 1, 2}
	c.blockByIdx = map[int]int{0: 0, 1: 1, 2: 2}

	if ok, term, err := c.lowerAtomic("LOCK", Instr{Raw: "LOCK"}); !ok || term || err != nil {
		t.Fatalf("lowerAtomic(LOCK) = (%v, %v, %v)", ok, term, err)
	}
	if ok, _, err := c.lowerAtomic("CMPXCHGL", Instr{
		Raw: "CMPXCHGL CX, 8(BX)",
		Args: []Operand{
			{Kind: OpReg, Reg: CX},
			{Kind: OpMem, Mem: MemRef{Base: BX, Off: 8}},
		},
	}); !ok || err != nil {
		t.Fatalf("lowerAtomic(CMPXCHGL) = (%v, %v)", ok, err)
	}
	if ok, _, err := c.lowerAtomic("XADDQ", Instr{
		Raw: "XADDQ AX, 16(BX)",
		Args: []Operand{
			{Kind: OpReg, Reg: AX},
			{Kind: OpMem, Mem: MemRef{Base: BX, Off: 16}},
		},
	}); !ok || err != nil {
		t.Fatalf("lowerAtomic(XADDQ) = (%v, %v)", ok, err)
	}
	if ok, _, err := c.lowerAtomic("XCHGB", Instr{
		Raw: "XCHGB AX, BX",
		Args: []Operand{
			{Kind: OpReg, Reg: AX},
			{Kind: OpReg, Reg: BX},
		},
	}); !ok || err != nil {
		t.Fatalf("lowerAtomic(XCHGB reg) = (%v, %v)", ok, err)
	}
	if ok, _, err := c.lowerAtomic("XCHGQ", Instr{
		Raw: "XCHGQ AX, global<>(SB)",
		Args: []Operand{
			{Kind: OpReg, Reg: AX},
			{Kind: OpSym, Sym: "global<>(SB)"},
		},
	}); !ok || err != nil {
		t.Fatalf("lowerAtomic(XCHGQ sym) = (%v, %v)", ok, err)
	}
	if ok, _, err := c.lowerAtomic("ANDB", Instr{
		Raw: "ANDB $1, 4(BX)",
		Args: []Operand{
			{Kind: OpImm, Imm: 1},
			{Kind: OpMem, Mem: MemRef{Base: BX, Off: 4}},
		},
	}); !ok || err != nil {
		t.Fatalf("lowerAtomic(ANDB) = (%v, %v)", ok, err)
	}
	if ok, term, err := c.lowerAtomic("ORQ", Instr{
		Raw: "ORQ AX, BX",
		Args: []Operand{
			{Kind: OpReg, Reg: AX},
			{Kind: OpReg, Reg: BX},
		},
	}); ok || term || err != nil {
		t.Fatalf("lowerAtomic(non-mem ORQ) = (%v, %v, %v)", ok, term, err)
	}
	if _, err := c.amd64AtomicTruncFromI64("%x", LLVMType("v2i64")); err == nil {
		t.Fatalf("amd64AtomicTruncFromI64(unsupported) unexpectedly succeeded")
	}
	if _, err := c.amd64AtomicExtendToI64("%x", LLVMType("v2i64")); err == nil {
		t.Fatalf("amd64AtomicExtendToI64(unsupported) unexpectedly succeeded")
	}

	emitBr := func(target string) {
		b.WriteString("  br label %" + amd64LLVMBlockName(target) + "\n")
	}
	emitCondBr := func(cond string, target string, fall string) error {
		b.WriteString("  br i1 " + cond + ", label %" + amd64LLVMBlockName(target) + ", label %" + amd64LLVMBlockName(fall) + "\n")
		return nil
	}
	if ok, term, err := c.lowerBranch(0, 0, "CALL", Instr{
		Raw:  "CALL helper(SB)",
		Args: []Operand{{Kind: OpSym, Sym: "helper(SB)"}},
	}, emitBr, emitCondBr); !ok || term || err != nil {
		t.Fatalf("lowerBranch(CALL sym) = (%v, %v, %v)", ok, term, err)
	}
	if ok, term, err := c.lowerBranch(0, 0, "CALL", Instr{
		Raw:  "CALL AX",
		Args: []Operand{{Kind: OpReg, Reg: AX}},
	}, emitBr, emitCondBr); !ok || term || err != nil {
		t.Fatalf("lowerBranch(CALL reg) = (%v, %v, %v)", ok, term, err)
	}
	if ok, term, err := c.lowerBranch(0, 0, "CALL", Instr{
		Raw:  "CALL 8(BX)",
		Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: BX, Off: 8}}},
	}, emitBr, emitCondBr); !ok || term || err != nil {
		t.Fatalf("lowerBranch(CALL mem) = (%v, %v, %v)", ok, term, err)
	}
	if ok, term, err := c.lowerBranch(0, 0, "JEQ", Instr{
		Raw:  "JEQ target",
		Args: []Operand{{Kind: OpIdent, Ident: "target"}},
	}, emitBr, emitCondBr); !ok || !term || err != nil {
		t.Fatalf("lowerBranch(JEQ ident) = (%v, %v, %v)", ok, term, err)
	}
	if ok, term, err := c.lowerBranch(0, 0, "JMP", Instr{
		Raw:  "JMP 2(PC)",
		Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: PC, Off: 2}}},
	}, emitBr, emitCondBr); !ok || !term || err != nil {
		t.Fatalf("lowerBranch(JMP pc-rel) = (%v, %v, %v)", ok, term, err)
	}
	if ok, term, err := c.lowerBranch(0, 0, "JMP", Instr{
		Raw:  "JMP AX",
		Args: []Operand{{Kind: OpReg, Reg: AX}},
	}, emitBr, emitCondBr); !ok || !term || err != nil {
		t.Fatalf("lowerBranch(JMP reg) = (%v, %v, %v)", ok, term, err)
	}
	if ok, _, err := c.lowerBranch(0, 0, "JEQ", Instr{
		Raw:  "JEQ AX",
		Args: []Operand{{Kind: OpReg, Reg: AX}},
	}, emitBr, emitCondBr); !ok || err == nil {
		t.Fatalf("lowerBranch(JEQ reg) = (%v, %v), want error", ok, err)
	}

	if err := c.callSym(Operand{Kind: OpSym, Sym: "runtime·entersyscall(SB)"}); err != nil {
		t.Fatalf("callSym(entersyscall) error = %v", err)
	}
	if err := c.callSym(Operand{Kind: OpSym, Sym: "helper(SB)"}); err != nil {
		t.Fatalf("callSym(helper) error = %v", err)
	}
	if err := c.callSym(Operand{Kind: OpSym, Sym: "badarg(SB)"}); err == nil {
		t.Fatalf("callSym(badarg) unexpectedly succeeded")
	}
	if err := c.callSym(Operand{Kind: OpSym, Sym: "badret(SB)"}); err == nil {
		t.Fatalf("callSym(badret) unexpectedly succeeded")
	}
	if err := c.callSym(Operand{Kind: OpReg, Reg: AX}); err == nil {
		t.Fatalf("callSym(non-sym) unexpectedly succeeded")
	}
	if err := c.callSym(Operand{Kind: OpSym, Sym: "missing"}); err == nil {
		t.Fatalf("callSym(missing suffix) unexpectedly succeeded")
	}

	if err := c.tailCallAndRet(Operand{Kind: OpSym, Sym: "cast(SB)"}); err != nil {
		t.Fatalf("tailCallAndRet(cast) error = %v", err)
	}
	if err := c.tailCallAndRet(Operand{Kind: OpSym, Sym: "helper(SB)"}); err != nil {
		t.Fatalf("tailCallAndRet(helper) error = %v", err)
	}
	cErr, _ := newAMD64CtxWithFuncForTest(t, fn, FuncSig{Name: "example.errcaller", Args: []LLVMType{I64}, Ret: I64}, sigs)
	if err := cErr.tailCallAndRet(Operand{Kind: OpSym, Sym: "voidret(SB)"}); err == nil {
		t.Fatalf("tailCallAndRet(voidret) unexpectedly succeeded")
	}
	if err := c.tailCallAndRet(Operand{Kind: OpReg, Reg: AX}); err == nil {
		t.Fatalf("tailCallAndRet(non-sym) unexpectedly succeeded")
	}
	if err := c.tailCallIndirectAddrAndRet("123"); err != nil {
		t.Fatalf("tailCallIndirectAddrAndRet() error = %v", err)
	}

	out := b.String()
	for _, want := range []string{
		"cmpxchg ptr",
		"atomicrmw add ptr",
		"atomicrmw xchg ptr",
		"atomicrmw and ptr",
		"call i64",
		"call ptr @\"example.helper\"",
		"call i64 @\"example.cast\"",
		"ret i64",
		"br i1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestAMD64ArithmeticCoverage(t *testing.T) {
	fn := Func{
		Instrs: []Instr{
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: DX}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: DI}, {Kind: OpReg, Reg: SI}}},
			{Op: "MOVQ", Args: []Operand{{Kind: OpReg, Reg: Reg("R8")}, {Kind: OpReg, Reg: Reg("R9")}}},
		},
	}
	sig := FuncSig{
		Name: "example.arith",
		Ret:  Void,
		Frame: FrameLayout{
			Results: []FrameSlot{
				{Offset: 8, Type: I1, Index: 0, Field: -1},
				{Offset: 16, Type: I64, Index: 1, Field: -1},
			},
		},
	}
	c, b := newAMD64CtxWithFuncForTest(t, fn, sig, nil)
	mustLower := func(op Op, ins Instr) {
		t.Helper()
		if ok, term, err := c.lowerArith(op, ins); !ok || term || err != nil {
			t.Fatalf("lowerArith(%s) = (%v, %v, %v)", op, ok, term, err)
		}
	}

	mustLower("PUSHQ", Instr{Raw: "PUSHQ $1", Args: []Operand{{Kind: OpImm, Imm: 1}}})
	mustLower("POPQ", Instr{Raw: "POPQ CX", Args: []Operand{{Kind: OpReg, Reg: CX}}})
	mustLower("PUSHFQ", Instr{Raw: "PUSHFQ"})
	mustLower("POPFQ", Instr{Raw: "POPFQ"})
	mustLower("LFENCE", Instr{Raw: "LFENCE"})
	mustLower("UNDEF", Instr{Raw: "UNDEF"})
	mustLower("RDTSC", Instr{Raw: "RDTSC"})
	mustLower(OpCPUID, Instr{Raw: "CPUID"})
	mustLower(OpXGETBV, Instr{Raw: "XGETBV"})
	mustLower("RDTSCP", Instr{Raw: "RDTSCP"})
	mustLower("MOVSB", Instr{Raw: "MOVSB"})
	mustLower("MOVSQ", Instr{Raw: "MOVSQ"})
	mustLower("STOSQ", Instr{Raw: "STOSQ"})
	mustLower("NEGL", Instr{Raw: "NEGL AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("NEGL", Instr{Raw: "NEGL 4(BX)", Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: BX, Off: 4}}}})
	mustLower("RCRQ", Instr{Raw: "RCRQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ADDQ", Instr{Raw: "ADDQ $2, AX", Args: []Operand{{Kind: OpImm, Imm: 2}, {Kind: OpReg, Reg: AX}}})
	mustLower("SUBQ", Instr{Raw: "SUBQ CX, 8(BX)", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpMem, Mem: MemRef{Base: BX, Off: 8}}}})
	mustLower("XORQ", Instr{Raw: "XORQ $3, AX", Args: []Operand{{Kind: OpImm, Imm: 3}, {Kind: OpReg, Reg: AX}}})
	mustLower("ANDQ", Instr{Raw: "ANDQ $4, AX", Args: []Operand{{Kind: OpImm, Imm: 4}, {Kind: OpReg, Reg: AX}}})
	mustLower("ORQ", Instr{Raw: "ORQ $5, AX", Args: []Operand{{Kind: OpImm, Imm: 5}, {Kind: OpReg, Reg: AX}}})
	mustLower("ADCQ", Instr{Raw: "ADCQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SBBQ", Instr{Raw: "SBBQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ADCXQ", Instr{Raw: "ADCXQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ADOXQ", Instr{Raw: "ADOXQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ADDL", Instr{Raw: "ADDL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SUBL", Instr{Raw: "SUBL CX, 12(BX)", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpMem, Mem: MemRef{Base: BX, Off: 12}}}})
	mustLower("XORL", Instr{Raw: "XORL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ANDL", Instr{Raw: "ANDL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ORL", Instr{Raw: "ORL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("XORB", Instr{Raw: "XORB $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ANDB", Instr{Raw: "ANDB $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ORB", Instr{Raw: "ORB $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("INCQ", Instr{Raw: "INCQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("DECQ", Instr{Raw: "DECQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("INCL", Instr{Raw: "INCL AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("DECL", Instr{Raw: "DECL AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("LEAQ", Instr{Raw: "LEAQ 8(BX)(CX*2), DI", Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: BX, Index: CX, Scale: 2, Off: 8}}, {Kind: OpReg, Reg: DI}}})
	mustLower("LEAL", Instr{Raw: "LEAL 4(BX), SI", Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: BX, Off: 4}}, {Kind: OpReg, Reg: SI}}})
	mustLower("LEAQ", Instr{Raw: "LEAQ ret+16(FP), R8", Args: []Operand{{Kind: OpFP, FPOffset: 16}, {Kind: OpReg, Reg: Reg("R8")}}})
	mustLower("LEAQ", Instr{Raw: "LEAQ $ret+16(FP), R9", Args: []Operand{{Kind: OpFPAddr, FPOffset: 16}, {Kind: OpReg, Reg: Reg("R9")}}})
	mustLower("LEAQ", Instr{Raw: "LEAQ global<>(SB), AX", Args: []Operand{{Kind: OpSym, Sym: "global<>(SB)"}, {Kind: OpReg, Reg: AX}}})
	mustLower("POPCNTL", Instr{Raw: "POPCNTL AX, BX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}})
	mustLower("POPCNTQ", Instr{Raw: "POPCNTQ AX, CX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: CX}}})
	mustLower("BSFQ", Instr{Raw: "BSFQ AX, BX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}})
	mustLower("BSRQ", Instr{Raw: "BSRQ AX, CX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: CX}}})
	mustLower("BSWAPQ", Instr{Raw: "BSWAPQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("BSFL", Instr{Raw: "BSFL AX, DX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: DX}}})
	mustLower("BSRL", Instr{Raw: "BSRL AX, DI", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: DI}}})
	mustLower("SETEQ", Instr{Raw: "SETEQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("SETGT", Instr{Raw: "SETGT ret+8(FP)", Args: []Operand{{Kind: OpFP, FPOffset: 8}}})
	mustLower("SETHI", Instr{Raw: "SETHI BX", Args: []Operand{{Kind: OpReg, Reg: BX}}})
	mustLower("CMOVQEQ", Instr{Raw: "CMOVQEQ CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("CMOVQNE", Instr{Raw: "CMOVQNE CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("CMOVQCS", Instr{Raw: "CMOVQCS CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("CMOVQCC", Instr{Raw: "CMOVQCC CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("CMOVQGT", Instr{Raw: "CMOVQGT CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("ANDNL", Instr{Raw: "ANDNL AX, BX, CX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}, {Kind: OpReg, Reg: CX}}})
	mustLower("ANDNQ", Instr{Raw: "ANDNQ AX, BX, DX", Args: []Operand{{Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}, {Kind: OpReg, Reg: DX}}})
	mustLower("SHRQ", Instr{Raw: "SHRQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SHLQ", Instr{Raw: "SHLQ CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("SARQ", Instr{Raw: "SARQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SHLL", Instr{Raw: "SHLL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SHRL", Instr{Raw: "SHRL CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("SALQ", Instr{Raw: "SALQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("SALL", Instr{Raw: "SALL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ROLL", Instr{Raw: "ROLL $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("ROLQ", Instr{Raw: "ROLQ CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("RORQ", Instr{Raw: "RORQ $1, AX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}}})
	mustLower("RORL", Instr{Raw: "RORL CX, AX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: AX}}})
	mustLower("RORXL", Instr{Raw: "RORXL $1, AX, BX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: BX}}})
	mustLower("RORXQ", Instr{Raw: "RORXQ $1, AX, CX", Args: []Operand{{Kind: OpImm, Imm: 1}, {Kind: OpReg, Reg: AX}, {Kind: OpReg, Reg: CX}}})
	mustLower("NOTL", Instr{Raw: "NOTL AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("NOTQ", Instr{Raw: "NOTQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("NOTQ", Instr{Raw: "NOTQ 20(BX)", Args: []Operand{{Kind: OpMem, Mem: MemRef{Base: BX, Off: 20}}}})
	mustLower("BSWAPL", Instr{Raw: "BSWAPL AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})
	mustLower("MULQ", Instr{Raw: "MULQ CX", Args: []Operand{{Kind: OpReg, Reg: CX}}})
	mustLower("MULXQ", Instr{Raw: "MULXQ BX, CX, DI", Args: []Operand{{Kind: OpReg, Reg: BX}, {Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: DI}}})
	mustLower("MULL", Instr{Raw: "MULL CX", Args: []Operand{{Kind: OpReg, Reg: CX}}})
	mustLower("DIVL", Instr{Raw: "DIVL CX", Args: []Operand{{Kind: OpReg, Reg: CX}}})
	mustLower("IMULQ", Instr{Raw: "IMULQ CX", Args: []Operand{{Kind: OpReg, Reg: CX}}})
	mustLower("IMULQ", Instr{Raw: "IMULQ CX, DX", Args: []Operand{{Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: DX}}})
	mustLower("IMUL3Q", Instr{Raw: "IMUL3Q $3, CX, DI", Args: []Operand{{Kind: OpImm, Imm: 3}, {Kind: OpReg, Reg: CX}, {Kind: OpReg, Reg: DI}}})
	mustLower("NEGQ", Instr{Raw: "NEGQ AX", Args: []Operand{{Kind: OpReg, Reg: AX}}})

	out := b.String()
	for _, want := range []string{
		`asm sideeffect "cpuid"`,
		`asm sideeffect "xgetbv"`,
		"call i32 @llvm.ctpop.i32",
		"call i64 @llvm.ctpop.i64",
		"call i64 @llvm.cttz.i64",
		"call i32 @llvm.ctlz.i32",
		"call i64 @llvm.bswap.i64",
		"call i32 @llvm.bswap.i32",
		"mul i128",
		"udiv i64",
		"urem i64",
		"select i1",
		"ptrtoint ptr @\"example.global\" to i64",
		"store i8",
		"ashr i64",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}
