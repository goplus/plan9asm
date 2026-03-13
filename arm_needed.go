package plan9asm

import "strings"

// funcNeedsARMCFG decides whether ARM lowering needs the CFG-based path.
// The linear prototype only handles straight-line arithmetic/data movement.
func funcNeedsARMCFG(fn Func) bool {
	for _, ins := range fn.Instrs {
		if ins.Op == OpLABEL {
			return true
		}
		rawOp := strings.ToUpper(string(ins.Op))
		op := rawOp
		if dot := strings.IndexByte(op, '.'); dot >= 0 {
			op = op[:dot]
		}
		switch Op(op) {
		case OpTEXT, OpRET, OpBYTE:
			continue
		case "MOVW", "MOVB", "MOVBU", "ADD", "SUB", "AND", "ORR", "EOR", "RSB":
			// The linear ARM path cannot handle conditional execution or post-inc.
			if strings.Contains(rawOp, ".") {
				return true
			}
			continue
		default:
			return true
		}
	}
	return false
}
