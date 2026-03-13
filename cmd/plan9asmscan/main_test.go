package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/goplus/plan9asm"
)

func TestToPlan9Arch(t *testing.T) {
	if got, err := toPlan9Arch("amd64"); err != nil || got != plan9asm.ArchAMD64 {
		t.Fatalf("toPlan9Arch(amd64) = (%q, %v)", got, err)
	}
	if got, err := toPlan9Arch("arm"); err != nil || got != plan9asm.ArchARM {
		t.Fatalf("toPlan9Arch(arm) = (%q, %v)", got, err)
	}
	if got, err := toPlan9Arch("arm64"); err != nil || got != plan9asm.ArchARM64 {
		t.Fatalf("toPlan9Arch(arm64) = (%q, %v)", got, err)
	}
	if _, err := toPlan9Arch("wasm"); err == nil {
		t.Fatalf("expected unsupported arch error")
	}
}

func TestNormalizeOpAndDirectiveHelpers(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "movd", want: "MOVD"},
		{in: "b.eq", want: "B"},
		{in: " foo ", want: "FOO"},
		{in: "CALL*", want: ""},
		{in: "foo_bar", want: ""},
	}
	for _, tc := range cases {
		if got := normalizeOp(tc.in); got != tc.want {
			t.Fatalf("normalizeOp(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	if !isDirective("TEXT") || !isDirective("DATA") {
		t.Fatalf("expected directive recognition")
	}
	if isDirective("MOVD") {
		t.Fatalf("MOVD should not be a directive")
	}
}

func TestClusterOfAndTopFiles(t *testing.T) {
	if got := clusterOf("amd64", "CALL"); got != "x86-control" {
		t.Fatalf("clusterOf amd64 CALL = %q", got)
	}
	if got := clusterOf("amd64", "VPXOR"); got != "x86-simd" {
		t.Fatalf("clusterOf amd64 VPXOR = %q", got)
	}
	if got := clusterOf("arm64", "BL"); got != "arm64-control" {
		t.Fatalf("clusterOf arm64 BL = %q", got)
	}
	if got := clusterOf("arm64", "CASAL"); got != "arm64-atomic" {
		t.Fatalf("clusterOf arm64 CASAL = %q", got)
	}
	if got := clusterOf("arm64", "VADD"); got != "arm64-neon" {
		t.Fatalf("clusterOf arm64 VADD = %q", got)
	}
	if got := clusterOf("other", "MOV"); got != "other" {
		t.Fatalf("clusterOf other MOV = %q", got)
	}

	got := topFiles(map[string]int{
		"b.s": 3,
		"c.s": 3,
		"a.s": 5,
	}, 2)
	if len(got) != 2 || got[0] != "a.s" || got[1] != "b.s" {
		t.Fatalf("topFiles = %#v", got)
	}
}

func TestShortStdPath(t *testing.T) {
	goroot := runtime.GOROOT()
	if goroot == "" {
		t.Skip("GOROOT not available")
	}

	inRoot := filepath.Join(goroot, "src", "runtime", "sys_arm64.s")
	if got := shortStdPath(inRoot); got != "runtime/sys_arm64.s" {
		t.Fatalf("shortStdPath(inRoot) = %q", got)
	}

	outside := filepath.Join(t.TempDir(), "local.s")
	if got := shortStdPath(outside); got != filepath.ToSlash(outside) {
		t.Fatalf("shortStdPath(outside) = %q", got)
	}
}

func TestExtractSupportedOpsScansRepoRoot(t *testing.T) {
	dir := t.TempDir()
	src := `package plan9asm

func lower(op string) {
	switch op {
	case "VPXORQ", "VMOVDQU64", "AESENC":
	}
}

func lowerOp(op any) {
	switch op {
	case OpMOVQ, OpRET:
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "amd64_lower_vec.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	supported, err := extractSupportedOps(dir, "amd64")
	if err != nil {
		t.Fatalf("extractSupportedOps: %v", err)
	}
	want := []string{"VPXORQ", "VMOVDQU64", "AESENC", "MOVQ", "RET"}
	got := make([]string, 0, len(want))
	for _, op := range want {
		if _, ok := supported[op]; ok {
			got = append(got, op)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("supported ops = %#v, want %#v", got, want)
	}
}

func TestFamilyOfAMD64(t *testing.T) {
	cases := map[string]string{
		"VPXORQ":         "avx-vector",
		"VGF2P8AFFINEQB": "gfni",
		"KMOVQ":          "avx512-mask",
		"AESENCLAST":     "aes",
		"SHA1MSG1":       "sha",
		"RORXQ":          "bmi2-adx",
		"JEQ":            "branch-alias",
		"MOVOA":          "sse-simd",
		"MOVLQZX":        "move-pseudo",
		"XADDQ":          "atomic-memory",
	}
	for op, want := range cases {
		if got := familyOf("amd64", op); got != want {
			t.Fatalf("familyOf(%q) = %q, want %q", op, got, want)
		}
	}
}
