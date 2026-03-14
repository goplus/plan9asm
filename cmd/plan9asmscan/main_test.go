package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestListStdPackages(t *testing.T) {
	pkgs, err := listStdPackages(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("listStdPackages() error = %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("listStdPackages() returned no packages")
	}
	foundRuntime := false
	for _, p := range pkgs {
		if p.ImportPath == "runtime" {
			foundRuntime = true
			break
		}
	}
	if !foundRuntime {
		t.Fatalf("listStdPackages() missing runtime package")
	}
}

func TestPackageSFilesAndAddOpStat(t *testing.T) {
	pkg := pkgJSON{
		ImportPath: "example/p",
		Dir:        "/tmp/pkg",
		SFiles:     []string{"a.s", "b.S", filepath.Join("/abs", "c.s")},
	}
	got := packageSFiles(pkg)
	want := []string{filepath.Join("/tmp/pkg", "a.s"), filepath.Join("/abs", "c.s")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packageSFiles() = %#v, want %#v", got, want)
	}

	ops := map[string]*opStat{}
	addOpStat(ops, "MOVD", "a.s", "example/p", 2)
	addOpStat(ops, "bad*", "a.s", "example/p", 2)
	addOpStat(ops, "RET", "a.s", "example/p", 1)
	addOpStat(ops, "MOVD", "a.s", "example/p", 0)
	if got := ops["MOVD"].Count; got != 2 {
		t.Fatalf("MOVD count = %d, want 2", got)
	}
	if got := ops["RET"].Count; got != 1 {
		t.Fatalf("RET count = %d, want 1", got)
	}
	if _, ok := ops["BAD"]; ok {
		t.Fatalf("invalid op unexpectedly added")
	}
}

func TestScanPackagesAndBuildReport(t *testing.T) {
	dir := t.TempDir()
	good := `TEXT ·f(SB),NOSPLIT,$0-0
	MOVQ $1, AX
	NOP
	RET
`
	bad := `DATA foo(SB), $1
`
	dataOnly := `TEXT ·datafn(SB),NOSPLIT,$0-0
	RET
DATA foo+0(SB)/8, $1
GLOBL foo(SB), RODATA, $8
`
	if err := os.WriteFile(filepath.Join(dir, "good.s"), []byte(good), 0o644); err != nil {
		t.Fatalf("write good.s: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.s"), []byte(bad), 0o644); err != nil {
		t.Fatalf("write bad.s: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.s"), []byte(dataOnly), 0o644); err != nil {
		t.Fatalf("write data.s: %v", err)
	}

	pkgs := []pkgJSON{{
		ImportPath: "example/p",
		Dir:        dir,
		SFiles:     []string{"good.s", "bad.s", "data.s"},
	}}
	ops, parseErrs, pkgsWithS, asmFiles, err := scanPackages(pkgs, plan9asm.ArchAMD64)
	if err != nil {
		t.Fatalf("scanPackages() error = %v", err)
	}
	if pkgsWithS != 1 {
		t.Fatalf("pkgWithSFiles = %d, want 1", pkgsWithS)
	}
	if asmFiles != 3 {
		t.Fatalf("asmFiles = %d, want 3", asmFiles)
	}
	if len(parseErrs) != 1 {
		t.Fatalf("parseErrs = %#v, want 1 entry", parseErrs)
	}
	for _, op := range []string{"MOVQ", "NOP", "RET", "DATA", "GLOBL"} {
		if _, ok := ops[op]; !ok {
			t.Fatalf("scanPackages() missing %q in ops %#v", op, ops)
		}
	}

	rep := buildReport("linux", "amd64", 10, pkgsWithS, asmFiles, ops, map[string]struct{}{
		"RET":  {},
		"MOVQ": {},
	}, parseErrs)
	if rep.Goos != "linux" || rep.Goarch != "amd64" {
		t.Fatalf("buildReport() wrong target: %#v", rep)
	}
	if rep.ParseErrCount != 1 || len(rep.ParseErrs) != 1 {
		t.Fatalf("buildReport() parse errs = %#v", rep.ParseErrs)
	}
	if len(rep.Unsupported) == 0 {
		t.Fatalf("buildReport() expected unsupported ops for NOP")
	}
	if len(rep.ClusterStats) == 0 || len(rep.FamilyStats) == 0 {
		t.Fatalf("buildReport() expected cluster/family stats")
	}

	md := string(renderMarkdown(rep))
	for _, want := range []string{
		"# Plan9 Asm Scan Report (linux/amd64)",
		"## Cluster Summary",
		"## Unsupported Ops (vs current lowerers)",
		"good.s",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("renderMarkdown() missing %q in:\n%s", want, md)
		}
	}
}

func TestBuildReportAndJSONShape(t *testing.T) {
	ops := map[string]*opStat{
		"CALL":   {Count: 3, Files: map[string]int{"a.s": 2}, Pkgs: map[string]int{"p": 3}},
		"VPXORQ": {Count: 5, Files: map[string]int{"b.s": 5}, Pkgs: map[string]int{"p": 5}},
		"DATA":   {Count: 1, Files: map[string]int{"c.s": 1}, Pkgs: map[string]int{"p": 1}},
	}
	rep := buildReport("linux", "amd64", 3, 1, 2, ops, map[string]struct{}{"CALL": {}}, []parseErr{{File: "bad.s", Err: "boom"}})
	if rep.UniqueOps != 3 {
		t.Fatalf("UniqueOps = %d, want 3", rep.UniqueOps)
	}
	if len(rep.OpsByFreq) != 3 {
		t.Fatalf("OpsByFreq len = %d, want 3", len(rep.OpsByFreq))
	}
	if rep.OpsByFreq[0].Op != "VPXORQ" {
		t.Fatalf("OpsByFreq[0] = %#v", rep.OpsByFreq[0])
	}
	if len(rep.Unsupported) != 1 {
		t.Fatalf("Unsupported len = %d, want 1", len(rep.Unsupported))
	}
	js, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal(report) error = %v", err)
	}
	if !strings.Contains(string(js), `"goarch":"amd64"`) {
		t.Fatalf("json output missing goarch: %s", js)
	}
}

func TestClusterAndFamilyCoverageMore(t *testing.T) {
	clusterCases := []struct {
		goarch string
		op     string
		want   string
	}{
		{"amd64", "RET", "x86-control"},
		{"amd64", "VZEROUPPER", "x86-simd"},
		{"amd64", "CRC32Q", "x86-crc"},
		{"amd64", "LOCK", "x86-atomic"},
		{"amd64", "BTQ", "x86-bit-shift"},
		{"amd64", "MOVQ", "x86-scalar"},
		{"arm64", "VADD", "arm64-neon"},
		{"arm64", "CBNZ", "arm64-control"},
		{"arm64", "SWPALD", "arm64-atomic"},
		{"arm64", "RBIT", "arm64-bit-shift"},
		{"arm64", "ADD", "arm64-scalar"},
		{"other", "MOV", "other"},
	}
	for _, tc := range clusterCases {
		if got := clusterOf(tc.goarch, tc.op); got != tc.want {
			t.Fatalf("clusterOf(%q, %q) = %q, want %q", tc.goarch, tc.op, got, tc.want)
		}
	}

	familyCases := []struct {
		goarch string
		op     string
		want   string
	}{
		{"amd64", "AESENC", "aes"},
		{"amd64", "SHA256MSG2", "sha"},
		{"amd64", "VGF2P8AFFINEQB", "gfni"},
		{"amd64", "KORW", "avx512-mask"},
		{"amd64", "VZEROALL", "avx-vector"},
		{"amd64", "MOVAPS", "sse-simd"},
		{"amd64", "ADCXQ", "bmi2-adx"},
		{"amd64", "MFENCE", "atomic-memory"},
		{"amd64", "JCS", "branch-alias"},
		{"amd64", "SHRQ", "bit-rotate-shift"},
		{"amd64", "ADJSP", "move-pseudo"},
		{"amd64", "ADDQ", "scalar-misc"},
		{"arm64", "AESD", "crypto"},
		{"arm64", "VEOR", "neon"},
		{"arm64", "TBZ", "branch"},
		{"arm64", "CASALD", "atomic-memory"},
		{"arm64", "ADD", "scalar-misc"},
		{"other", "X", "other"},
	}
	for _, tc := range familyCases {
		if got := familyOf(tc.goarch, tc.op); got != tc.want {
			t.Fatalf("familyOf(%q, %q) = %q, want %q", tc.goarch, tc.op, got, tc.want)
		}
	}
}

func TestMainAndFatalfSubprocess(t *testing.T) {
	testBin := os.Args[0]
	if os.Getenv("PLAN9ASMSCAN_MAIN_HELPER") == "1" {
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = []string{
			"plan9asmscan",
			"-goos=linux",
			"-goarch=amd64",
			"-format=json",
			"-repo-root=../..",
			"-out=" + filepath.Join(os.TempDir(), "plan9asmscan-test.json"),
		}
		main()
		return
	}
	if os.Getenv("PLAN9ASMSCAN_FATALF_HELPER") == "1" {
		fatalf("boom %d", 7)
		return
	}

	outPath := filepath.Join(t.TempDir(), "report.json")
	helperHome := t.TempDir()
	oldArgs := os.Args
	oldFlags := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldFlags
	}()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{
		"plan9asmscan",
		"-goos=linux",
		"-goarch=amd64",
		"-format=json",
		"-repo-root=../..",
		"-out=" + outPath,
	}
	main()
	if data, err := os.ReadFile(outPath); err != nil || !strings.Contains(string(data), `"goarch": "amd64"`) {
		t.Fatalf("main() output = (%q, %v)", string(data), err)
	}

	cmd := exec.Command(testBin, "-test.run=TestMainAndFatalfSubprocess")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + helperHome,
		"GOCACHE=" + filepath.Join(helperHome, "gocache"),
		"PLAN9ASMSCAN_MAIN_HELPER=1",
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main helper failed: %v\n%s", err, out)
	}

	fcmd := exec.Command(testBin, "-test.run=TestMainAndFatalfSubprocess")
	fcmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + helperHome,
		"GOCACHE=" + filepath.Join(helperHome, "gocache"),
		"PLAN9ASMSCAN_FATALF_HELPER=1",
	}
	fout, err := fcmd.CombinedOutput()
	if err == nil {
		t.Fatalf("fatalf helper unexpectedly succeeded")
	}
	if !strings.Contains(string(fout), "boom 7") {
		t.Fatalf("fatalf output = %q, want boom 7", string(fout))
	}
}
