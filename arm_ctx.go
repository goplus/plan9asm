package plan9asm

import (
	"fmt"
	"sort"
	"strings"
)

type armCtx struct {
	b        *strings.Builder
	sig      FuncSig
	resolve  func(string) string
	sigs     map[string]FuncSig
	annotate bool

	tmp int

	blocks []armBlock

	usedRegs map[Reg]bool
	regSlot  map[Reg]string

	flagsNSlot   string
	flagsZSlot   string
	flagsCSlot   string
	flagsVSlot   string
	flagsWritten bool

	fpParams       map[int64]FrameSlot
	fpResults      []FrameSlot
	fpResAllocaOff map[int64]string
	fpResAllocaIdx map[int]string
	fpResWritten   map[int]bool
	fpResAddrTaken map[int]bool
}

func newARMCtx(b *strings.Builder, fn Func, sig FuncSig, resolve func(string) string, sigs map[string]FuncSig, annotate bool) *armCtx {
	c := &armCtx{
		b:              b,
		sig:            sig,
		resolve:        resolve,
		sigs:           sigs,
		annotate:       annotate,
		blocks:         armSplitBlocks(fn),
		usedRegs:       map[Reg]bool{},
		regSlot:        map[Reg]string{},
		fpParams:       map[int64]FrameSlot{},
		fpResAllocaOff: map[int64]string{},
		fpResAllocaIdx: map[int]string{},
		fpResWritten:   map[int]bool{},
		fpResAddrTaken: map[int]bool{},
	}
	for _, s := range sig.Frame.Params {
		c.fpParams[s.Offset] = s
	}
	c.fpResults = append([]FrameSlot(nil), sig.Frame.Results...)
	return c
}

func (c *armCtx) emitSourceComment(ins Instr) {
	if !c.annotate {
		return
	}
	emitIRSourceComment(c.b, ins.Raw)
}

func (c *armCtx) newTmp() string {
	c.tmp++
	return fmt.Sprintf("t%d", c.tmp)
}

func (c *armCtx) slotName(r Reg) string {
	return "%" + armLLVMBlockName("reg_"+string(r))
}

func (c *armCtx) scanUsedRegs() {
	markReg := func(r Reg) {
		if r != "" {
			c.usedRegs[r] = true
		}
	}
	markOp := func(op Operand) {
		switch op.Kind {
		case OpReg, OpRegShift:
			markReg(op.Reg)
			markReg(op.ShiftReg)
		case OpMem:
			markReg(op.Mem.Base)
			markReg(op.Mem.Index)
		case OpRegList:
			for _, r := range op.RegList {
				markReg(r)
			}
		}
	}
	for _, blk := range c.blocks {
		for _, ins := range blk.instrs {
			for _, op := range ins.Args {
				markOp(op)
			}
		}
	}
	if len(c.sig.ArgRegs) > 0 {
		for i := 0; i < len(c.sig.Args) && i < len(c.sig.ArgRegs); i++ {
			markReg(c.sig.ArgRegs[i])
		}
	} else {
		for i := 0; i < len(c.sig.Args) && i < 4; i++ {
			markReg(Reg(fmt.Sprintf("R%d", i)))
		}
	}
	for i := 0; i <= 15; i++ {
		markReg(Reg(fmt.Sprintf("R%d", i)))
	}
	markReg(SP)
}

func (c *armCtx) emitEntryAllocasAndArgInit() error {
	c.scanUsedRegs()
	regs := make([]string, 0, len(c.usedRegs))
	for r := range c.usedRegs {
		regs = append(regs, string(r))
	}
	sort.Strings(regs)

	c.b.WriteString(armLLVMBlockName("entry") + ":\n")
	for _, rs := range regs {
		r := Reg(rs)
		slot := c.slotName(r)
		c.regSlot[r] = slot
		fmt.Fprintf(c.b, "  %s = alloca i32\n", slot)
		fmt.Fprintf(c.b, "  store i32 0, ptr %s\n", slot)
	}

	c.flagsNSlot = "%flags_n"
	c.flagsZSlot = "%flags_z"
	c.flagsCSlot = "%flags_c"
	c.flagsVSlot = "%flags_v"
	fmt.Fprintf(c.b, "  %s = alloca i1\n", c.flagsNSlot)
	fmt.Fprintf(c.b, "  store i1 false, ptr %s\n", c.flagsNSlot)
	fmt.Fprintf(c.b, "  %s = alloca i1\n", c.flagsZSlot)
	fmt.Fprintf(c.b, "  store i1 false, ptr %s\n", c.flagsZSlot)
	fmt.Fprintf(c.b, "  %s = alloca i1\n", c.flagsCSlot)
	fmt.Fprintf(c.b, "  store i1 false, ptr %s\n", c.flagsCSlot)
	fmt.Fprintf(c.b, "  %s = alloca i1\n", c.flagsVSlot)
	fmt.Fprintf(c.b, "  store i1 false, ptr %s\n", c.flagsVSlot)

	for _, r := range c.fpResults {
		name := fmt.Sprintf("%%fp_ret_%d", r.Index)
		c.fpResAllocaIdx[r.Index] = name
		c.fpResAllocaOff[r.Offset] = name
		fmt.Fprintf(c.b, "  %s = alloca %s\n", name, r.Type)
		fmt.Fprintf(c.b, "  store %s %s, ptr %s\n", r.Type, llvmZeroValue(r.Type), name)
	}

	if len(c.sig.ArgRegs) > 0 {
		for i := 0; i < len(c.sig.Args) && i < len(c.sig.ArgRegs); i++ {
			slot, ok := c.regSlot[c.sig.ArgRegs[i]]
			if !ok {
				continue
			}
			v, ok, err := armValueAsI32(c, c.sig.Args[i], fmt.Sprintf("%%arg%d", i))
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			fmt.Fprintf(c.b, "  store i32 %s, ptr %s\n", v, slot)
		}
	}
	return nil
}

func armValueAsI32(c *armCtx, ty LLVMType, v string) (out string, ok bool, err error) {
	switch ty {
	case Ptr:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = ptrtoint ptr %s to i32\n", t, v)
		return "%" + t, true, nil
	case I1:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i1 %s to i32\n", t, v)
		return "%" + t, true, nil
	case I8:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i8 %s to i32\n", t, v)
		return "%" + t, true, nil
	case I16:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i16 %s to i32\n", t, v)
		return "%" + t, true, nil
	case I32:
		return v, true, nil
	case I64:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = trunc i64 %s to i32\n", t, v)
		return "%" + t, true, nil
	default:
		return "", false, nil
	}
}

func (c *armCtx) loadReg(r Reg) (string, error) {
	slot, ok := c.regSlot[r]
	if !ok {
		return "", fmt.Errorf("arm: unknown reg %s", r)
	}
	t := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = load i32, ptr %s\n", t, slot)
	return "%" + t, nil
}

func (c *armCtx) storeReg(r Reg, v string) error {
	slot, ok := c.regSlot[r]
	if !ok {
		return fmt.Errorf("arm: unknown reg %s", r)
	}
	fmt.Fprintf(c.b, "  store i32 %s, ptr %s\n", v, slot)
	return nil
}

func (c *armCtx) ptrFromSB(sym string) (string, error) {
	base, off, ok := parseSBRef(sym)
	if !ok {
		return "", fmt.Errorf("invalid (SB) sym ref: %q", sym)
	}
	base = strings.TrimPrefix(base, "$")
	res := base
	if strings.Contains(base, "·") || strings.Contains(base, "/") || strings.Contains(base, ".") {
		res = c.resolve(base)
	} else {
		res = c.resolve("·" + base)
	}
	p := llvmGlobal(res)
	if off == 0 {
		return p, nil
	}
	t := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = getelementptr i8, ptr %s, i32 %d\n", t, p, off)
	return "%" + t, nil
}
