package plan9asm

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *armCtx) imm32(n int64) string {
	return strconv.FormatInt(n, 10)
}

func (c *armCtx) addrI32(mem MemRef, postInc bool) (addr string, base Reg, inc int64, err error) {
	base = mem.Base
	baseVal, err := c.loadReg(base)
	if err != nil {
		return "", "", 0, err
	}
	off := mem.Off
	if postInc {
		inc = off
		off = 0
	}
	sum := baseVal
	if mem.Index != "" {
		idxVal, err := c.loadReg(mem.Index)
		if err != nil {
			return "", "", 0, err
		}
		if mem.Scale != 0 && mem.Scale != 1 {
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = mul i32 %s, %s\n", t, idxVal, c.imm32(mem.Scale))
			idxVal = "%" + t
		}
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %s\n", t, sum, idxVal)
		sum = "%" + t
	}
	if off != 0 {
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %s\n", t, sum, c.imm32(off))
		sum = "%" + t
	}
	return sum, base, inc, nil
}

func (c *armCtx) updatePostInc(base Reg, inc int64) error {
	if inc == 0 {
		return nil
	}
	baseVal, err := c.loadReg(base)
	if err != nil {
		return err
	}
	t := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = add i32 %s, %s\n", t, baseVal, c.imm32(inc))
	return c.storeReg(base, "%"+t)
}

func (c *armCtx) loadMem(mem MemRef, bits int, postInc bool, signed bool) (string, error) {
	addr, base, inc, err := c.addrI32(mem, postInc)
	if err != nil {
		return "", err
	}
	pt := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", pt, addr)
	ptr := "%" + pt

	var out string
	switch bits {
	case 32:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = load i32, ptr %s\n", t, ptr)
		out = "%" + t
	case 8:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = load i8, ptr %s\n", t, ptr)
		e := c.newTmp()
		if signed {
			fmt.Fprintf(c.b, "  %%%s = sext i8 %%%s to i32\n", e, t)
		} else {
			fmt.Fprintf(c.b, "  %%%s = zext i8 %%%s to i32\n", e, t)
		}
		out = "%" + e
	default:
		return "", fmt.Errorf("arm: unsupported load bits %d", bits)
	}
	if err := c.updatePostInc(base, inc); err != nil {
		return "", err
	}
	return out, nil
}

func (c *armCtx) storeMem(mem MemRef, bits int, postInc bool, v32 string) error {
	addr, base, inc, err := c.addrI32(mem, postInc)
	if err != nil {
		return err
	}
	pt := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", pt, addr)
	ptr := "%" + pt
	switch bits {
	case 32:
		fmt.Fprintf(c.b, "  store i32 %s, ptr %s\n", v32, ptr)
	case 8:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = trunc i32 %s to i8\n", t, v32)
		fmt.Fprintf(c.b, "  store i8 %%%s, ptr %s\n", t, ptr)
	default:
		return fmt.Errorf("arm: unsupported store bits %d", bits)
	}
	return c.updatePostInc(base, inc)
}

func (c *armCtx) eval32(op Operand, postInc bool) (string, error) {
	switch op.Kind {
	case OpImm:
		return c.imm32(op.Imm), nil
	case OpReg:
		return c.loadReg(op.Reg)
	case OpRegShift:
		return c.evalShift(op)
	case OpFP:
		return c.evalFPValue32(op)
	case OpFPAddr:
		return c.evalFPAddr32(op)
	case OpMem:
		return c.loadMem(op.Mem, 32, postInc, false)
	case OpSym:
		sym := strings.TrimSpace(op.Sym)
		if strings.HasPrefix(sym, "$") {
			sym = strings.TrimPrefix(sym, "$")
		}
		if mem, ok := parseMem(sym); ok {
			addr, _, _, err := c.addrI32(mem, false)
			if err != nil {
				return "", err
			}
			return addr, nil
		}
		p, err := c.ptrFromSB(sym)
		if err != nil {
			return "", err
		}
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = ptrtoint ptr %s to i32\n", t, p)
		return "%" + t, nil
	case OpIdent:
		return "0", nil
	default:
		return "", fmt.Errorf("arm: unsupported operand for i32: %s", op.String())
	}
}

func (c *armCtx) evalShift(op Operand) (string, error) {
	base, err := c.loadReg(op.Reg)
	if err != nil {
		return "", err
	}
	var sh string
	if op.ShiftReg != "" {
		sh, err = c.loadReg(op.ShiftReg)
		if err != nil {
			return "", err
		}
	} else {
		sh = c.imm32(op.ShiftAmount)
	}
	t := c.newTmp()
	switch op.ShiftOp {
	case ShiftLeft:
		fmt.Fprintf(c.b, "  %%%s = shl i32 %s, %s\n", t, base, sh)
	case ShiftRight:
		fmt.Fprintf(c.b, "  %%%s = lshr i32 %s, %s\n", t, base, sh)
	case ShiftArith:
		fmt.Fprintf(c.b, "  %%%s = ashr i32 %s, %s\n", t, base, sh)
	case ShiftRotate:
		fmt.Fprintf(c.b, "  %%%s = call i32 @llvm.fshr.i32(i32 %s, i32 %s, i32 %s)\n", t, base, base, sh)
	default:
		return "", fmt.Errorf("arm: unsupported shift op %q", op.ShiftOp)
	}
	return "%" + t, nil
}

func (c *armCtx) evalFPValue32(op Operand) (string, error) {
	slot, ok := c.fpParams[op.FPOffset]
	if !ok {
		return "", fmt.Errorf("arm: unsupported FP param slot: %s", op.String())
	}
	if slot.Index < 0 || slot.Index >= len(c.sig.Args) {
		return "", fmt.Errorf("arm: FP slot %s invalid arg index %d", op.String(), slot.Index)
	}
	arg := fmt.Sprintf("%%arg%d", slot.Index)
	if slot.Field >= 0 {
		aggTy := c.sig.Args[slot.Index]
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = extractvalue %s %s, %d\n", t, aggTy, arg, slot.Field)
		arg = "%" + t
	}
	switch slot.Type {
	case Ptr:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = ptrtoint ptr %s to i32\n", t, arg)
		return "%" + t, nil
	case I1:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i1 %s to i32\n", t, arg)
		return "%" + t, nil
	case I8:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i8 %s to i32\n", t, arg)
		return "%" + t, nil
	case I16:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i16 %s to i32\n", t, arg)
		return "%" + t, nil
	case I32:
		return arg, nil
	case I64:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = trunc i64 %s to i32\n", t, arg)
		return "%" + t, nil
	default:
		return "", fmt.Errorf("arm: unsupported FP slot type %q", slot.Type)
	}
}

func (c *armCtx) evalFPAddr32(op Operand) (string, error) {
	if slot, ok := c.fpResAllocaOff[op.FPOffset]; ok {
		c.markFPResultAddrTaken(op.FPOffset)
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = ptrtoint ptr %s to i32\n", t, slot)
		return "%" + t, nil
	}
	return "", fmt.Errorf("arm: unsupported FP addr slot: %s", op.String())
}
