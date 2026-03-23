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

func TestStdlibInternalRuntimeSys_ARM64_Compile(t *testing.T) {
	llc, _, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc not found")
	}
	const triple = "aarch64-unknown-linux-gnu"

	goroot := testGOROOT(t)
	src, err := os.ReadFile(filepath.Join(goroot, "src", "internal", "runtime", "sys", "dit_arm64.s"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("internal/runtime/sys/dit_arm64.s not present in this GOROOT")
		}
		t.Fatal(err)
	}

	file, err := Parse(ArchARM64, string(src))
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(sym string) string {
		if strings.HasPrefix(sym, "·") {
			return "internal/runtime/sys." + strings.TrimPrefix(sym, "·")
		}
		return strings.ReplaceAll(sym, "·", ".")
	}
	sigs := map[string]FuncSig{
		"internal/runtime/sys.EnableDIT": {
			Name: "internal/runtime/sys.EnableDIT",
			Ret:  I1,
			Frame: FrameLayout{
				Results: []FrameSlot{{Offset: 0, Type: I1, Index: 0}},
			},
		},
		"internal/runtime/sys.DITEnabled": {
			Name: "internal/runtime/sys.DITEnabled",
			Ret:  I1,
			Frame: FrameLayout{
				Results: []FrameSlot{{Offset: 0, Type: I1, Index: 0}},
			},
		},
		"internal/runtime/sys.DisableDIT": {
			Name: "internal/runtime/sys.DisableDIT",
			Ret:  Void,
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
	llPath := filepath.Join(tmp, "dit.ll")
	objPath := filepath.Join(tmp, "dit.o")
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

func TestTranslateGoModule_StdlibInternalRuntimeSys_ARM64_Compile(t *testing.T) {
	llc, _, ok := findLlcAndClang(t)
	if !ok {
		t.Skip("llc not found")
	}
	const triple = "aarch64-unknown-linux-gnu"

	goroot := testGOROOT(t)
	src, err := os.ReadFile(filepath.Join(goroot, "src", "internal", "runtime", "sys", "dit_arm64.s"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("internal/runtime/sys/dit_arm64.s not present in this GOROOT")
		}
		t.Fatal(err)
	}
	pkg := mustGoPackage(t, "internal/runtime/sys", `package sys
func EnableDIT() bool
func DITEnabled() bool
func DisableDIT()
`)

	tr, err := TranslateGoModule(pkg, src, GoModuleOptions{
		FileName:     "dit_arm64.s",
		GOOS:         "linux",
		GOARCH:       "arm64",
		TargetTriple: triple,
		ResolveSym:   testResolveSym("internal/runtime/sys"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Module.Dispose()

	tmp := t.TempDir()
	llPath := filepath.Join(tmp, "dit-gomod.ll")
	objPath := filepath.Join(tmp, "dit-gomod.o")
	if err := os.WriteFile(llPath, []byte(tr.Module.String()), 0644); err != nil {
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

func testGOROOT(t *testing.T) string {
	t.Helper()
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		goroot = testGoEnv(t, "GOROOT")
	}
	if goroot == "" {
		t.Skip("GOROOT not available")
	}
	return goroot
}

func testGoEnv(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", key).CombinedOutput()
	if err != nil {
		t.Fatalf("go env %s failed: %v\n%s", key, err, string(out))
	}
	return strings.TrimSpace(string(out))
}
