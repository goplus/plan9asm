package plan9asm

import "fmt"

func (c *armCtx) lowerArith(op, cond string, ins Instr) (ok bool, terminated bool, err error) {
	switch op {
	case "ADD", "SUB", "AND", "ORR", "EOR", "RSB", "BIC":
		return true, false, c.lowerARMALU(op, cond, ins)
	case "MVN":
		return true, false, c.lowerARMMVN(cond, ins)
	case "CMP", "CMN", "TST", "TEQ":
		return true, false, c.lowerARMCompare(op, ins)
	}
	return false, false, nil
}

func (c *armCtx) lowerARMALU(op, cond string, ins Instr) error {
	if len(ins.Args) != 2 && len(ins.Args) != 3 {
		return fmt.Errorf("arm %s expects 2 or 3 operands: %q", op, ins.Raw)
	}
	var src, lhs string
	dst := Operand{}
	var err error
	if len(ins.Args) == 2 {
		dst = ins.Args[1]
		if dst.Kind != OpReg {
			return fmt.Errorf("arm %s dst must be reg: %q", op, ins.Raw)
		}
		src, err = c.eval32(ins.Args[0], false)
		if err != nil {
			return err
		}
		lhs, err = c.loadReg(dst.Reg)
		if err != nil {
			return err
		}
	} else {
		dst = ins.Args[2]
		if dst.Kind != OpReg {
			return fmt.Errorf("arm %s dst must be reg: %q", op, ins.Raw)
		}
		src, err = c.eval32(ins.Args[0], false)
		if err != nil {
			return err
		}
		lhs, err = c.eval32(ins.Args[1], false)
		if err != nil {
			return err
		}
	}
	t := c.newTmp()
	switch op {
	case "ADD":
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %s\n", t, lhs, src)
	case "SUB":
		fmt.Fprintf(c.b, "  %%%s = sub i32 %s, %s\n", t, lhs, src)
	case "AND":
		fmt.Fprintf(c.b, "  %%%s = and i32 %s, %s\n", t, lhs, src)
	case "ORR":
		fmt.Fprintf(c.b, "  %%%s = or i32 %s, %s\n", t, lhs, src)
	case "EOR":
		fmt.Fprintf(c.b, "  %%%s = xor i32 %s, %s\n", t, lhs, src)
	case "RSB":
		fmt.Fprintf(c.b, "  %%%s = sub i32 %s, %s\n", t, src, lhs)
	case "BIC":
		n := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = xor i32 %s, -1\n", n, src)
		fmt.Fprintf(c.b, "  %%%s = and i32 %s, %%%s\n", t, lhs, n)
	}
	return c.selectRegWrite(dst.Reg, cond, "%"+t)
}

func (c *armCtx) lowerARMCompare(op string, ins Instr) error {
	if len(ins.Args) != 2 {
		return fmt.Errorf("arm %s expects 2 operands: %q", op, ins.Raw)
	}
	src, err := c.eval32(ins.Args[0], false)
	if err != nil {
		return err
	}
	lhs, err := c.eval32(ins.Args[1], false)
	if err != nil {
		return err
	}
	res := c.newTmp()
	switch op {
	case "CMP":
		fmt.Fprintf(c.b, "  %%%s = sub i32 %s, %s\n", res, lhs, src)
		c.setFlagsSub(lhs, src, "%"+res)
	case "CMN":
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %s\n", res, lhs, src)
		c.setFlagsAdd(lhs, src, "%"+res)
	case "TST":
		fmt.Fprintf(c.b, "  %%%s = and i32 %s, %s\n", res, lhs, src)
		c.setFlagsLogic("%" + res)
	case "TEQ":
		fmt.Fprintf(c.b, "  %%%s = xor i32 %s, %s\n", res, lhs, src)
		c.setFlagsLogic("%" + res)
	}
	return nil
}

func (c *armCtx) lowerARMMVN(cond string, ins Instr) error {
	if len(ins.Args) != 2 && len(ins.Args) != 3 {
		return fmt.Errorf("arm MVN expects 2 or 3 operands: %q", ins.Raw)
	}
	var src string
	var dst Operand
	var err error
	if len(ins.Args) == 2 {
		src, err = c.eval32(ins.Args[0], false)
		if err != nil {
			return err
		}
		dst = ins.Args[1]
	} else {
		src, err = c.eval32(ins.Args[1], false)
		if err != nil {
			return err
		}
		dst = ins.Args[2]
	}
	if dst.Kind != OpReg {
		return fmt.Errorf("arm MVN dst must be reg: %q", ins.Raw)
	}
	t := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = xor i32 %s, -1\n", t, src)
	return c.selectRegWrite(dst.Reg, cond, "%"+t)
}
