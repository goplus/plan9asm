package plan9asm

import (
	"fmt"
)

func (c *amd64Ctx) evalI64(op Operand) (string, error) {
	switch op.Kind {
	case OpImm:
		return fmt.Sprintf("%d", op.Imm), nil
	case OpReg:
		return c.loadReg(op.Reg)
	case OpFP:
		return c.evalFPToI64(op.FPOffset)
	case OpMem:
		addr, err := c.addrFromMem(op.Mem)
		if err != nil {
			return "", err
		}
		p := c.ptrFromAddrI64(addr)
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = load i64, ptr %s, align 1\n", t, p)
		return "%" + t, nil
	case OpSym:
		p, err := c.ptrFromSB(op.Sym)
		if err != nil {
			return "", err
		}
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = load i64, ptr %s, align 1\n", t, p)
		return "%" + t, nil
	default:
		return "", fmt.Errorf("amd64: unsupported i64 operand: %s", op.String())
	}
}
