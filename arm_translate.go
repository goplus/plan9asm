package plan9asm

import "strings"

func emitARMPrelude(b *strings.Builder) {
	b.WriteString("declare i32 @cliteErrno()\n")
	b.WriteString("declare i32 @llvm.fshr.i32(i32, i32, i32)\n")
	b.WriteString("\n")
}

func armLLVMBlockName(src string) string {
	return arm64LLVMBlockName(src)
}
