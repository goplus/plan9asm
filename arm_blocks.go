package plan9asm

import (
	"fmt"
	"strings"
)

type armBlock struct {
	name   string
	instrs []Instr
}

var armCondCodes = map[string]bool{
	"EQ": true,
	"NE": true,
	"CS": true,
	"HS": true,
	"CC": true,
	"LO": true,
	"MI": true,
	"PL": true,
	"VS": true,
	"VC": true,
	"HI": true,
	"LS": true,
	"GE": true,
	"LT": true,
	"GT": true,
	"LE": true,
}

func armSplitBlocks(fn Func) []armBlock {
	blocks := []armBlock{{name: "entry"}}
	cur := 0
	anon := 0

	startAnon := func() {
		anon++
		blocks = append(blocks, armBlock{name: fmt.Sprintf("anon_%d", anon)})
		cur = len(blocks) - 1
	}

	isTerminator := func(ins Instr) bool {
		if ins.Op == OpRET {
			return true
		}
		baseOp, cond, _, _ := armDecodeOp(strings.ToUpper(string(ins.Op)))
		switch baseOp {
		case "B", "JMP":
			return true
		case "BEQ", "BNE", "BLT", "BGE", "BGT", "BLE", "BHS", "BHI", "BLS", "BLO", "BCC", "BCS", "BMI":
			return true
		}
		return baseOp == "B" && cond != ""
	}

	for _, ins := range fn.Instrs {
		if ins.Op == OpLABEL && len(ins.Args) == 1 && ins.Args[0].Kind == OpLabel {
			lbl := ins.Args[0].Sym
			if len(blocks[cur].instrs) == 0 && strings.HasPrefix(blocks[cur].name, "anon_") {
				blocks[cur].name = lbl
				continue
			}
			blocks = append(blocks, armBlock{name: lbl})
			cur = len(blocks) - 1
			continue
		}

		blocks[cur].instrs = append(blocks[cur].instrs, ins)
		if isTerminator(ins) {
			startAnon()
		}
	}

	if len(blocks) > 1 && len(blocks[len(blocks)-1].instrs) == 0 && strings.HasPrefix(blocks[len(blocks)-1].name, "anon_") {
		blocks = blocks[:len(blocks)-1]
	}
	return blocks
}

func armBranchTarget(op Operand) (string, bool) {
	switch op.Kind {
	case OpIdent:
		return op.Ident, true
	case OpSym:
		s := strings.TrimSuffix(strings.TrimSuffix(op.Sym, "(SB)"), "<>")
		if s == "" {
			return "", false
		}
		return s, true
	default:
		return "", false
	}
}

func armDecodeOp(raw string) (base string, cond string, postInc bool, setFlags bool) {
	base = strings.ToUpper(strings.TrimSpace(raw))
	if base == "" {
		return "", "", false, false
	}
	parts := strings.Split(base, ".")
	base = parts[0]
	for _, p := range parts[1:] {
		switch p {
		case "P":
			postInc = true
		case "S":
			setFlags = true
		case "":
		default:
			if armCondCodes[p] {
				cond = p
			}
		}
	}
	return base, cond, postInc, setFlags
}
