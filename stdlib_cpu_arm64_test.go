//go:build !llgo
// +build !llgo

package plan9asm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStdlibInternalCPU_ARM64_Compile(t *testing.T) {
	llc, _, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc not found")
	}
	const triple = "aarch64-unknown-linux-gnu"

	goroot := testGOROOT(t)
	src, err := os.ReadFile(filepath.Join(goroot, "src", "internal", "cpu", "cpu_arm64.s"))
	if err != nil {
		t.Fatal(err)
	}

	file, err := Parse(ArchARM64, string(src))
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(sym string) string {
		if strings.HasPrefix(sym, "·") {
			return "internal/cpu." + strings.TrimPrefix(sym, "·")
		}
		return strings.ReplaceAll(sym, "·", ".")
	}
	sigs := map[string]FuncSig{
		"internal/cpu.getisar0": {
			Name:  "internal/cpu.getisar0",
			Ret:   I64,
			Frame: FrameLayout{Results: []FrameSlot{{Offset: 0, Type: I64, Index: 0}}},
		},
		"internal/cpu.getpfr0": {
			Name:  "internal/cpu.getpfr0",
			Ret:   I64,
			Frame: FrameLayout{Results: []FrameSlot{{Offset: 0, Type: I64, Index: 0}}},
		},
		"internal/cpu.getMIDR": {
			Name:  "internal/cpu.getMIDR",
			Ret:   I64,
			Frame: FrameLayout{Results: []FrameSlot{{Offset: 0, Type: I64, Index: 0}}},
		},
	}
	ll, err := Translate(file, Options{
		TargetTriple: triple,
		ResolveSym:   resolve,
		Sigs:         sigs,
		Goarch:       "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	llPath := filepath.Join(tmp, "cpu.ll")
	objPath := filepath.Join(tmp, "cpu.o")
	if err := os.WriteFile(llPath, []byte(ll), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(llc, "-mtriple="+triple, "-filetype=obj", llPath, "-o", objPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := string(out)
		if llcUnsupportedTarget(s) {
			t.Skipf("llc does not support triple %q: %s", triple, strings.TrimSpace(s))
		}
		t.Fatalf("llc failed: %v\n%s", err, s)
	}
}
