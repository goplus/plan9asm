package plan9asm

import (
	"fmt"
	"strings"
)

func armDecodeMOVM(raw string) (mode string, writeback bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "IAW") || strings.Contains(s, "IA.W"):
		return "IA", true
	case strings.Contains(s, "DBW") || strings.Contains(s, "DB.W"):
		return "DB", true
	case strings.Contains(s, "WP"):
		return "DB", true
	case strings.Contains(s, ".IA"):
		return "IA", false
	case strings.Contains(s, ".IB"):
		return "IB", false
	case strings.Contains(s, ".DB"):
		return "DB", false
	case strings.Contains(s, ".DA"):
		return "DA", false
	default:
		return "IA", false
	}
}

func armRegListAllGPR(regs []Reg) bool {
	for _, r := range regs {
		p, _, ok := regRangeParts(r)
		if !ok || p != "R" {
			return false
		}
	}
	return true
}

func (c *armCtx) lowerMOVM(rawOp string, ins Instr) (ok bool, terminated bool, err error) {
	if !strings.HasPrefix(rawOp, "MOVM") {
		return false, false, nil
	}
	if len(ins.Args) != 2 {
		return true, false, fmt.Errorf("arm MOVM expects 2 operands: %q", ins.Raw)
	}
	mode, writeback := armDecodeMOVM(rawOp)
	var mem MemRef
	var regs []Reg
	load := false
	switch {
	case ins.Args[0].Kind == OpMem && ins.Args[1].Kind == OpRegList:
		mem = ins.Args[0].Mem
		regs = ins.Args[1].RegList
		load = true
	case ins.Args[0].Kind == OpRegList && ins.Args[1].Kind == OpMem:
		mem = ins.Args[1].Mem
		regs = ins.Args[0].RegList
	default:
		return true, false, fmt.Errorf("arm MOVM expects mem/reglist operands: %q", ins.Raw)
	}
	if !armRegListAllGPR(regs) {
		return true, false, fmt.Errorf("arm MOVM currently supports only general-purpose reg lists: %q", ins.Raw)
	}
	baseAddr, baseReg, _, err := c.addrI32(mem, false)
	if err != nil {
		return true, false, err
	}
	count := int64(len(regs))
	start := baseAddr
	switch mode {
	case "IA":
	case "IB":
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, 4\n", t, start)
		start = "%" + t
	case "DB":
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %d\n", t, start, -4*count)
		start = "%" + t
	case "DA":
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = add i32 %s, %d\n", t, start, -4*(count-1))
		start = "%" + t
	default:
		return true, false, fmt.Errorf("arm MOVM unsupported addressing mode %q: %q", mode, ins.Raw)
	}

	addr := start
	for i, r := range regs {
		pt := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", pt, addr)
		ptr := "%" + pt
		if load {
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = load i32, ptr %s\n", t, ptr)
			if err := c.storeReg(r, "%"+t); err != nil {
				return true, false, err
			}
		} else {
			v, err := c.loadReg(r)
			if err != nil {
				return true, false, err
			}
			fmt.Fprintf(c.b, "  store i32 %s, ptr %s\n", v, ptr)
		}
		if i+1 < len(regs) {
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = add i32 %s, 4\n", t, addr)
			addr = "%" + t
		}
	}

	if writeback {
		final := baseAddr
		switch mode {
		case "IA", "IB":
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = add i32 %s, %d\n", t, baseAddr, 4*count)
			final = "%" + t
		case "DB", "DA":
			t := c.newTmp()
			fmt.Fprintf(c.b, "  %%%s = add i32 %s, %d\n", t, baseAddr, -4*count)
			final = "%" + t
		}
		if err := c.storeReg(baseReg, final); err != nil {
			return true, false, err
		}
	}
	return true, false, nil
}
