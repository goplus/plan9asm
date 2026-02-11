//go:build !llgo
// +build !llgo

package plan9asm

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeExecAMD64Add(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("runtime execution test only runs on amd64 host")
	}

	llc, clang, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc/clang not found")
	}

	src := `
TEXT add2(SB),NOSPLIT,$0-0
	MOVQ a+0(FP), AX
	ADDQ b+8(FP), AX
	MOVQ AX, ret+16(FP)
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}

	ll, err := Translate(file, Options{
		TargetTriple: testTargetTriple(runtime.GOOS, runtime.GOARCH),
		Sigs: map[string]FuncSig{
			"add2": {
				Name: "add2",
				Args: []LLVMType{I64, I64},
				Ret:  I64,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I64, Index: 0, Field: -1},
						{Offset: 8, Type: I64, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 16, Type: I64, Index: 0, Field: -1},
					},
				},
			},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}

	mainC := `
#include <stdint.h>
extern long long add2(long long a, long long b);
int main(void) {
	long long got = add2(7, 35);
	return got == 42 ? 0 : 11;
}
`
	compileAndRunRuntimeTest(t, llc, clang, "amd64_add2", ll, mainC)
}

func TestRuntimeExecARM64Add(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("runtime execution test only runs on arm64 host")
	}

	llc, clang, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc/clang not found")
	}

	src := `
TEXT add2(SB),NOSPLIT,$0-0
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	ADD R1, R0
	MOVD R0, ret+16(FP)
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}

	ll, err := Translate(file, Options{
		TargetTriple: testTargetTriple(runtime.GOOS, runtime.GOARCH),
		Sigs: map[string]FuncSig{
			"add2": {
				Name: "add2",
				Args: []LLVMType{I64, I64},
				Ret:  I64,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I64, Index: 0, Field: -1},
						{Offset: 8, Type: I64, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 16, Type: I64, Index: 0, Field: -1},
					},
				},
			},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}

	mainC := `
#include <stdint.h>
extern long long add2(long long a, long long b);
int main(void) {
	long long got = add2(9, 33);
	return got == 42 ? 0 : 12;
}
`
	compileAndRunRuntimeTest(t, llc, clang, "arm64_add2", ll, mainC)
}

func TestRuntimeExecAMD64AddlJLE(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("runtime execution test only runs on amd64 host")
	}

	llc, clang, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc/clang not found")
	}

	src := `
TEXT addlJLE(SB),NOSPLIT,$0-0
	MOVL a+0(FP), AX
	ADDL b+8(FP), AX
	JLE le
	MOVQ $0, AX
	MOVQ AX, ret+16(FP)
	RET
le:
	MOVQ $1, AX
	MOVQ AX, ret+16(FP)
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}

	ll, err := Translate(file, Options{
		TargetTriple: testTargetTriple(runtime.GOOS, runtime.GOARCH),
		Sigs: map[string]FuncSig{
			"addlJLE": {
				Name: "addlJLE",
				Args: []LLVMType{I64, I64},
				Ret:  I64,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I64, Index: 0, Field: -1},
						{Offset: 8, Type: I64, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 16, Type: I64, Index: 0, Field: -1},
					},
				},
			},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}

	mainC := `
#include <stdint.h>
extern long long addlJLE(long long a, long long b);
int main(void) {
	if (addlJLE(-2, 1) != 1) return 31;
	if (addlJLE(0, 1) != 0) return 32;
	if (addlJLE(-1, 1) != 1) return 33;
	return 0;
}
`
	compileAndRunRuntimeTest(t, llc, clang, "amd64_addl_jle", ll, mainC)
}

func TestRuntimeExecAMD64AddlJB(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("runtime execution test only runs on amd64 host")
	}

	llc, clang, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc/clang not found")
	}

	src := `
TEXT addlJB(SB),NOSPLIT,$0-0
	MOVL a+0(FP), AX
	ADDL b+8(FP), AX
	JB carry
	MOVQ $0, AX
	MOVQ AX, ret+16(FP)
	RET
carry:
	MOVQ $1, AX
	MOVQ AX, ret+16(FP)
	RET
`
	file, err := Parse(ArchAMD64, src)
	if err != nil {
		t.Fatal(err)
	}

	ll, err := Translate(file, Options{
		TargetTriple: testTargetTriple(runtime.GOOS, runtime.GOARCH),
		Sigs: map[string]FuncSig{
			"addlJB": {
				Name: "addlJB",
				Args: []LLVMType{I64, I64},
				Ret:  I64,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I64, Index: 0, Field: -1},
						{Offset: 8, Type: I64, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 16, Type: I64, Index: 0, Field: -1},
					},
				},
			},
		},
		Goarch: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}

	mainC := `
#include <stdint.h>
extern long long addlJB(long long a, long long b);
int main(void) {
	if (addlJB(0xFFFFFFFFLL, 1) != 1) return 41;
	if (addlJB(1, 1) != 0) return 42;
	return 0;
}
`
	compileAndRunRuntimeTest(t, llc, clang, "amd64_addl_jb", ll, mainC)
}

func TestRuntimeExecARM64CmpBLE(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("runtime execution test only runs on arm64 host")
	}

	llc, clang, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc/clang not found")
	}

	src := `
TEXT cmpBLE(SB),NOSPLIT,$0-0
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	CMP R1, R0
	BLE le
	MOVD $0, R0
	MOVD R0, ret+16(FP)
	RET
le:
	MOVD $1, R0
	MOVD R0, ret+16(FP)
	RET
`
	file, err := Parse(ArchARM64, src)
	if err != nil {
		t.Fatal(err)
	}

	ll, err := Translate(file, Options{
		TargetTriple: testTargetTriple(runtime.GOOS, runtime.GOARCH),
		Sigs: map[string]FuncSig{
			"cmpBLE": {
				Name: "cmpBLE",
				Args: []LLVMType{I64, I64},
				Ret:  I64,
				Frame: FrameLayout{
					Params: []FrameSlot{
						{Offset: 0, Type: I64, Index: 0, Field: -1},
						{Offset: 8, Type: I64, Index: 1, Field: -1},
					},
					Results: []FrameSlot{
						{Offset: 16, Type: I64, Index: 0, Field: -1},
					},
				},
			},
		},
		Goarch: "arm64",
	})
	if err != nil {
		t.Fatal(err)
	}

	mainC := `
#include <stdint.h>
extern long long cmpBLE(long long a, long long b);
int main(void) {
	if (cmpBLE(1, 2) != 1) return 51;
	if (cmpBLE(2, 1) != 0) return 52;
	if (cmpBLE(-1, -1) != 1) return 53;
	return 0;
}
`
	compileAndRunRuntimeTest(t, llc, clang, "arm64_cmp_ble", ll, mainC)
}

func compileAndRunRuntimeTest(t *testing.T, llc, clang, name, ll, mainC string) {
	t.Helper()

	tmp := t.TempDir()
	llPath := filepath.Join(tmp, name+".ll")
	objPath := filepath.Join(tmp, name+".o")
	mainPath := filepath.Join(tmp, "main.c")
	exePath := filepath.Join(tmp, "a.out")
	triple := testTargetTriple(runtime.GOOS, runtime.GOARCH)

	if err := os.WriteFile(llPath, []byte(ll), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(mainC), 0644); err != nil {
		t.Fatal(err)
	}

	llcCmd := exec.Command(llc, "-mtriple="+triple, "-filetype=obj", llPath, "-o", objPath)
	if out, err := llcCmd.CombinedOutput(); err != nil {
		s := string(out)
		if llcUnsupportedTarget(s) {
			t.Skipf("llc does not support triple %q: %s", triple, strings.TrimSpace(s))
		}
		t.Fatalf("llc failed: %v\n%s", err, s)
	}

	clangCmd := exec.Command(clang, objPath, mainPath, "-O2", "-o", exePath)
	if out, err := clangCmd.CombinedOutput(); err != nil {
		t.Fatalf("clang link failed: %v\n%s", err, string(out))
	}

	runCmd := exec.Command(exePath)
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(out))
	}
}

func llcUnsupportedTarget(s string) bool {
	return strings.Contains(s, "No available targets") ||
		strings.Contains(s, "no targets are registered") ||
		strings.Contains(s, "unknown target triple") ||
		strings.Contains(s, "unknown target") ||
		strings.Contains(s, "is not a registered target")
}
