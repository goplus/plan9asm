package plan9asm

import (
	"fmt"
	"os"

	"github.com/goplus/llvm"
)

// TranslateModule converts a parsed Plan 9 asm File into an llvm.Module.
//
// Caller owns the returned module and should call Dispose when finished.
func TranslateModule(file *File, opt Options) (llvm.Module, error) {
	ir, err := translateIRText(file, opt)
	if err != nil {
		return llvm.Module{}, err
	}
	return parseIRModule(ir)
}

func parseIRModule(ir string) (llvm.Module, error) {
	f, err := os.CreateTemp("", "plan9asm-*.ll")
	if err != nil {
		return llvm.Module{}, fmt.Errorf("create temp ir file: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	defer os.Remove(name)

	if err := os.WriteFile(name, []byte(ir), 0644); err != nil {
		return llvm.Module{}, fmt.Errorf("write temp ir file: %w", err)
	}
	buf, err := llvm.NewMemoryBufferFromFile(name)
	if err != nil {
		return llvm.Module{}, fmt.Errorf("open temp ir file: %w", err)
	}
	// NOTE: do not dispose MemoryBuffer here. In this llvm binding, ParseIR
	// may take ownership and disposing the buffer can crash.
	ctx := llvm.GlobalContext()
	mod, err := (&ctx).ParseIR(buf)
	if err != nil {
		return llvm.Module{}, fmt.Errorf("parse generated ir: %w", err)
	}
	return mod, nil
}
