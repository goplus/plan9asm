package plan9asm

import (
	"fmt"
	"strings"
)

func translateFuncARM(b *strings.Builder, fn Func, sig FuncSig, resolve func(string) string, sigs map[string]FuncSig, annotateSource bool) error {
	fmt.Fprintf(b, "define %s %s(", sig.Ret, llvmGlobal(sig.Name))
	for i, t := range sig.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s %%arg%d", t, i)
	}
	b.WriteString(")")
	if sig.Attrs != "" {
		b.WriteString(" " + sig.Attrs)
	}
	b.WriteString(" {\n")

	c := newARMCtx(b, fn, sig, resolve, sigs, annotateSource)
	if err := c.emitEntryAllocasAndArgInit(); err != nil {
		return err
	}
	if err := c.lowerBlocks(); err != nil {
		return err
	}

	b.WriteString("}\n")
	return nil
}

func (c *armCtx) lowerBlocks() error {
	emitBr := func(target string) {
		fmt.Fprintf(c.b, "  br label %%%s\n", armLLVMBlockName(target))
	}
	emitCondBr := func(cond string, target string, fall string) error {
		cv, err := c.condValue(cond)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.b, "  br i1 %s, label %%%s, label %%%s\n", cv, armLLVMBlockName(target), armLLVMBlockName(fall))
		return nil
	}

	for bi := 0; bi < len(c.blocks); bi++ {
		blk := c.blocks[bi]
		if bi != 0 {
			fmt.Fprintf(c.b, "\n%s:\n", armLLVMBlockName(blk.name))
		}
		terminated := false
		for _, ins := range blk.instrs {
			c.emitSourceComment(ins)
			term, err := c.lowerInstr(bi, ins, emitBr, emitCondBr)
			if err != nil {
				return err
			}
			if term {
				terminated = true
				break
			}
		}
		if terminated {
			continue
		}
		if bi+1 < len(c.blocks) {
			emitBr(c.blocks[bi+1].name)
			continue
		}
		c.lowerRetZero()
	}
	return nil
}

func (c *armCtx) lowerInstr(bi int, ins Instr, emitBr armEmitBr, emitCondBr armEmitCondBr) (bool, error) {
	rawOp := strings.ToUpper(string(ins.Op))
	baseOp, cond, postInc := armDecodeOp(rawOp)
	switch baseOp {
	case string(OpTEXT), string(OpBYTE):
		return false, nil
	case string(OpRET):
		return true, c.lowerRET()
	case "PCDATA", "FUNCDATA", "NO_LOCAL_POINTERS", "WORD", "NOP", "DMB", "#IFDEF", "#ELSE", "#ENDIF":
		return false, nil
	}
	if ok, term, err := c.lowerData(baseOp, cond, postInc, ins); ok {
		return term, err
	}
	if ok, term, err := c.lowerArith(baseOp, cond, ins); ok {
		return term, err
	}
	if ok, term, err := c.lowerBranch(bi, baseOp, cond, ins, emitBr, emitCondBr); ok {
		return term, err
	}
	return false, fmt.Errorf("arm: unsupported instruction %s", ins.Op)
}

func (c *armCtx) lowerRET() error {
	if len(c.fpResults) == 0 {
		r0, err := c.loadReg(Reg("R0"))
		if err != nil {
			return err
		}
		switch c.sig.Ret {
		case Void:
			c.b.WriteString("  ret void\n")
		case I1, I8, I16:
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = trunc i32 %s to %s\n", t, r0, c.sig.Ret)
			fmt.Fprintf(c.b, "  ret %s %%%s\n", c.sig.Ret, t)
		case I32:
			fmt.Fprintf(c.b, "  ret i32 %s\n", r0)
		case Ptr:
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", t, r0)
			fmt.Fprintf(c.b, "  ret ptr %%%s\n", t)
		default:
			return fmt.Errorf("arm: unsupported return type %s", c.sig.Ret)
		}
		return nil
	}
	if len(c.fpResults) == 1 {
		slot := c.fpResults[0]
		var v string
		var err error
		if c.fpResWritten[slot.Index] || c.fpResAddrTaken[slot.Index] {
			v, err = c.loadFPResult(slot)
		} else {
			v, err = c.loadRetSlotFallback(slot)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(c.b, "  ret %s %s\n", c.sig.Ret, v)
		return nil
	}
	cur := "undef"
	last := ""
	for _, slot := range c.fpResults {
		var v string
		var err error
		if c.fpResWritten[slot.Index] || c.fpResAddrTaken[slot.Index] {
			v, err = c.loadFPResult(slot)
		} else {
			v, err = c.loadRetSlotFallback(slot)
		}
		if err != nil {
			return err
		}
		name := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = insertvalue %s %s, %s %s, %d\n", name, c.sig.Ret, cur, slot.Type, v, slot.Index)
		cur = "%" + name
		last = cur
	}
	fmt.Fprintf(c.b, "  ret %s %s\n", c.sig.Ret, last)
	return nil
}

func (c *armCtx) lowerRetZero() {
	switch c.sig.Ret {
	case Void:
		c.b.WriteString("  ret void\n")
	case I1:
		c.b.WriteString("  ret i1 false\n")
	case Ptr:
		c.b.WriteString("  ret ptr null\n")
	default:
		fmt.Fprintf(c.b, "  ret %s 0\n", c.sig.Ret)
	}
}
