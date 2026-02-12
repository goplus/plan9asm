package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/goplus/plan9asm"
	"golang.org/x/tools/go/packages"
)

type goListPackage struct {
	Dir        string   `json:"Dir"`
	ImportPath string   `json:"ImportPath"`
	SFiles     []string `json:"SFiles"`
}

type functionInfo struct {
	Symbol   string   `json:"symbol"`
	Resolved string   `json:"resolved"`
	Args     []string `json:"args"`
	Ret      string   `json:"ret"`
}

type translation struct {
	LLVMIR    string
	Functions []functionInfo
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		usage()
		return
	}
	switch cmd {
	case "list":
		check(runList(os.Args[2:]))
	case "transpile":
		check(runTranspile(os.Args[2:]))
	default:
		if strings.HasPrefix(cmd, "-") {
			check(runTranspile(os.Args[1:]))
			return
		}
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/plan9asm list [-goos <goos>] [-goarch <goarch>] [<pkg-or-path>]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/plan9asm transpile -pkg <pkg-or-path> -dir <out-dir> [-goos <goos>] [-goarch <goarch>] [flags]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/plan9asm transpile -i <file.s> -o <file.ll> [-goos <goos>] [-goarch <goarch>] [flags]")
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		goos   string
		goarch string
	)
	fs.StringVar(&goos, "goos", runtime.GOOS, "target GOOS")
	fs.StringVar(&goarch, "goarch", runtime.GOARCH, "target GOARCH (amd64/arm64/386)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := toPlan9Arch(goarch); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("list accepts at most one argument: <pkg or path>")
	}

	query := "std"
	if len(rest) == 1 {
		query = strings.TrimSpace(rest[0])
		if query == "" {
			return fmt.Errorf("empty <pkg or path>")
		}
	}

	pkgs, err := goListPackages(query, goos, goarch)
	if err != nil {
		return err
	}

	withSFiles := make([]goListPackage, 0, len(pkgs))
	for _, p := range pkgs {
		if len(p.SFiles) > 0 {
			withSFiles = append(withSFiles, p)
		}
	}
	sort.Slice(withSFiles, func(i, j int) bool {
		return withSFiles[i].ImportPath < withSFiles[j].ImportPath
	})

	for i, p := range withSFiles {
		fmt.Println(p.ImportPath)
		files := packageSFilesAbs(p)
		sort.Strings(files)
		for _, s := range files {
			fmt.Printf("  %s\n", s)
		}
		if i != len(withSFiles)-1 {
			fmt.Println()
		}
	}
	return nil
}

func runTranspile(args []string) error {
	fs := flag.NewFlagSet("transpile", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		pkgPath  string
		outDir   string
		inFile   string
		outFile  string
		goos     string
		goarch   string
		metaFile string
		annotate bool
	)
	fs.BoolVar(&annotate, "annotate", true, "emit source asm lines as IR comments")
	fs.StringVar(&inFile, "i", "", "Plan9 asm .s file path")
	fs.StringVar(&outFile, "o", "", "output .ll file path")
	fs.StringVar(&goarch, "goarch", runtime.GOARCH, "target GOARCH (amd64/arm64/386)")
	fs.StringVar(&goos, "goos", runtime.GOOS, "target GOOS")
	fs.StringVar(&metaFile, "meta", "", "optional output metadata json path")
	fs.StringVar(&pkgPath, "pkg", "", "Go package import path or filesystem path")
	fs.StringVar(&outDir, "dir", "", "output directory for package transpile")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := toPlan9Arch(goarch); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}

	pkgMode := strings.TrimSpace(pkgPath) != "" || strings.TrimSpace(outDir) != ""
	fileMode := strings.TrimSpace(inFile) != "" || strings.TrimSpace(outFile) != ""

	if pkgMode && fileMode {
		return fmt.Errorf("-pkg/-dir and -i/-o are mutually exclusive")
	}
	if !pkgMode && !fileMode {
		return fmt.Errorf("missing mode: use -pkg <pkg-or-path> -dir <out-dir>, or -i <file.s> -o <file.ll>")
	}

	if pkgMode {
		if strings.TrimSpace(pkgPath) == "" || strings.TrimSpace(outDir) == "" {
			return fmt.Errorf("package mode requires both -pkg and -dir")
		}
		return transpilePackageMode(pkgPath, outDir, goos, goarch, annotate, metaFile)
	}

	if strings.TrimSpace(inFile) == "" || strings.TrimSpace(outFile) == "" {
		return fmt.Errorf("single-file mode requires both -i and -o")
	}
	return transpileSingleFileMode(inFile, outFile, goos, goarch, annotate, metaFile)
}

func transpilePackageMode(pkgQuery, outDir, goos, goarch string, annotate bool, metaFile string) error {
	gpkg, err := selectSinglePackage(pkgQuery, goos, goarch)
	if err != nil {
		return err
	}
	pkg, err := loadPackage(gpkg.ImportPath, goos, goarch)
	if err != nil {
		return err
	}
	sfiles := packageSFilesAbs(gpkg)
	if len(sfiles) == 0 {
		return fmt.Errorf("package %s has no selected .s files for GOOS=%s GOARCH=%s", gpkg.ImportPath, goos, goarch)
	}
	sort.Strings(sfiles)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	type metaItem struct {
		File      string         `json:"file"`
		Output    string         `json:"output"`
		Functions []functionInfo `json:"functions"`
	}
	items := make([]metaItem, 0, len(sfiles))

	for _, sfile := range sfiles {
		tr, ok, err := translateAsmForPackage(pkg, sfile, goos, goarch, annotate)
		if err != nil {
			return fmt.Errorf("transpile %s: %w", sfile, err)
		}
		if !ok {
			continue
		}
		outPath := filepath.Join(outDir, filepath.Base(sfile)+".ll")
		if err := writeTextFile(outPath, tr.LLVMIR); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote llvm ir: %s\n", outPath)
		items = append(items, metaItem{File: sfile, Output: outPath, Functions: tr.Functions})
	}

	if metaFile != "" {
		payload := struct {
			Package string     `json:"package"`
			GOOS    string     `json:"goos"`
			GOARCH  string     `json:"goarch"`
			Items   []metaItem `json:"items"`
		}{
			Package: gpkg.ImportPath,
			GOOS:    goos,
			GOARCH:  goarch,
			Items:   items,
		}
		if err := writeJSONFile(metaFile, payload); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote metadata: %s\n", metaFile)
	}
	return nil
}

func transpileSingleFileMode(inFile, outFile, goos, goarch string, annotate bool, metaFile string) error {
	asmPath, err := resolvePath(inFile)
	if err != nil {
		return err
	}
	gpkg, err := selectSinglePackage(filepath.Dir(asmPath), goos, goarch)
	if err != nil {
		return err
	}
	pkg, err := loadPackage(gpkg.ImportPath, goos, goarch)
	if err != nil {
		return err
	}

	tr, ok, err := translateAsmForPackage(pkg, asmPath, goos, goarch, annotate)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s has no TEXT directive", asmPath)
	}
	if err := writeTextFile(outFile, tr.LLVMIR); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote llvm ir: %s\n", outFile)

	if metaFile != "" {
		payload := struct {
			Package   string         `json:"package"`
			File      string         `json:"file"`
			GOOS      string         `json:"goos"`
			GOARCH    string         `json:"goarch"`
			Functions []functionInfo `json:"functions"`
		}{
			Package:   pkg.PkgPath,
			File:      asmPath,
			GOOS:      goos,
			GOARCH:    goarch,
			Functions: tr.Functions,
		}
		if err := writeJSONFile(metaFile, payload); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote metadata: %s\n", metaFile)
	}
	return nil
}

func translateAsmForPackage(pkg *packages.Package, asmPath, goos, goarch string, annotate bool) (translation, bool, error) {
	src, err := os.ReadFile(asmPath)
	if err != nil {
		return translation{}, false, err
	}
	arch, err := toPlan9Arch(goarch)
	if err != nil {
		return translation{}, false, err
	}
	file, err := plan9asm.Parse(arch, string(src))
	if err != nil {
		if strings.Contains(err.Error(), "no TEXT directive found") {
			return translation{}, false, nil
		}
		return translation{}, false, err
	}
	if len(file.Funcs) == 0 {
		return translation{}, false, nil
	}

	resolve := resolveSymFunc(pkg.PkgPath)
	sigs, err := sigsForAsmFile(pkg, file, resolve, goarch)
	if err != nil {
		return translation{}, false, err
	}
	ll, err := plan9asm.Translate(file, plan9asm.Options{
		TargetTriple:   targetTriple(goos, goarch),
		ResolveSym:     resolve,
		Sigs:           sigs,
		Goarch:         goarch,
		AnnotateSource: annotate,
	})
	if err != nil {
		return translation{}, false, err
	}
	fns := collectFunctionInfo(file, sigs, resolve)
	return translation{LLVMIR: ll, Functions: fns}, true, nil
}

func collectFunctionInfo(file *plan9asm.File, sigs map[string]plan9asm.FuncSig, resolve func(string) string) []functionInfo {
	seen := map[string]bool{}
	out := make([]functionInfo, 0, len(file.Funcs))
	for _, fn := range file.Funcs {
		resolved := resolve(stripABISuffix(fn.Sym))
		if resolved == "" || seen[resolved] {
			continue
		}
		seen[resolved] = true
		sig := sigs[resolved]
		args := make([]string, len(sig.Args))
		for i, a := range sig.Args {
			args[i] = string(a)
		}
		ret := string(sig.Ret)
		if ret == "" {
			ret = string(plan9asm.Void)
		}
		out = append(out, functionInfo{
			Symbol:   fn.Sym,
			Resolved: resolved,
			Args:     args,
			Ret:      ret,
		})
	}
	return out
}

func toPlan9Arch(goarch string) (plan9asm.Arch, error) {
	switch goarch {
	case "amd64", "386":
		return plan9asm.ArchAMD64, nil
	case "arm64":
		return plan9asm.ArchARM64, nil
	default:
		return "", fmt.Errorf("unsupported -goarch %q (expect amd64/arm64/386)", goarch)
	}
}

func targetTriple(goos, goarch string) string {
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-macosx"
		case "arm64":
			return "arm64-apple-macosx"
		case "386":
			return "i386-apple-macosx"
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu"
		case "arm64":
			return "aarch64-unknown-linux-gnu"
		case "386":
			return "i386-unknown-linux-gnu"
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc"
		case "arm64":
			return "aarch64-pc-windows-msvc"
		case "386":
			return "i686-pc-windows-msvc"
		}
	}
	return ""
}

func loadPackage(pkgPath, goos, goarch string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedDeps |
			packages.NeedImports |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesSizes |
			packages.NeedTypesInfo,
		Env: append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch),
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("package not found: %s", pkgPath)
	}
	for _, p := range pkgs {
		if len(p.Errors) != 0 {
			return nil, fmt.Errorf("load %s: %s", p.PkgPath, p.Errors[0].Msg)
		}
	}
	for _, p := range pkgs {
		if p.PkgPath == pkgPath {
			if p.Types == nil || p.Types.Scope() == nil {
				return nil, fmt.Errorf("package %s loaded without types", pkgPath)
			}
			return p, nil
		}
	}
	p := pkgs[0]
	if p.Types == nil || p.Types.Scope() == nil {
		return nil, fmt.Errorf("package %s loaded without types", p.PkgPath)
	}
	return p, nil
}

func resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", abs)
	}
	return abs, nil
}

func goListPackages(query, goos, goarch string) ([]goListPackage, error) {
	args := []string{"list", "-json", query}
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr != "" {
			return nil, fmt.Errorf("go list -json %s failed: %w\n%s", query, err, stderr)
		}
		return nil, fmt.Errorf("go list -json %s failed: %w", query, err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []goListPackage
	for {
		var p goListPackage
		if err := dec.Decode(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse go list output for %q: %w", query, err)
		}
		if strings.TrimSpace(p.ImportPath) == "" {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func packageSFilesAbs(p goListPackage) []string {
	files := make([]string, 0, len(p.SFiles))
	for _, f := range p.SFiles {
		if filepath.IsAbs(f) {
			files = append(files, f)
		} else if p.Dir != "" {
			files = append(files, filepath.Join(p.Dir, f))
		}
	}
	return files
}

func selectSinglePackage(query, goos, goarch string) (goListPackage, error) {
	pkgs, err := goListPackages(query, goos, goarch)
	if err != nil {
		return goListPackage{}, err
	}
	if len(pkgs) == 0 {
		return goListPackage{}, fmt.Errorf("package not found: %s", query)
	}
	if len(pkgs) == 1 {
		return pkgs[0], nil
	}

	absQuery := query
	if filepath.IsAbs(query) {
		absQuery = filepath.Clean(query)
	}
	for _, p := range pkgs {
		if p.ImportPath == query {
			return p, nil
		}
		if absQuery != "" && p.Dir != "" && filepath.Clean(p.Dir) == absQuery {
			return p, nil
		}
	}

	paths := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		paths = append(paths, p.ImportPath)
	}
	sort.Strings(paths)
	return goListPackage{}, fmt.Errorf("query %q matches multiple packages: %s", query, strings.Join(paths, ", "))
}

func writeTextFile(path, data string) error {
	if path == "-" {
		_, err := os.Stdout.WriteString(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0644)
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeTextFile(path, string(data))
}

func resolveSymFunc(pkgPath string) func(sym string) string {
	return func(sym string) string {
		sym = stripABISuffix(sym)
		hadLocal := strings.HasSuffix(sym, "<>")
		sym = strings.TrimSuffix(sym, "<>")
		if strings.HasPrefix(sym, "·") {
			name := pkgPath + "." + strings.TrimPrefix(sym, "·")
			if hadLocal {
				return name + "$local"
			}
			return name
		}
		sym = strings.ReplaceAll(sym, "∕", "/")
		sym = strings.ReplaceAll(sym, "·", ".")
		if hadLocal {
			if !strings.Contains(sym, "/") && !strings.Contains(sym, ".") {
				return pkgPath + "." + sym + "$local"
			}
			return sym + "$local"
		}
		return sym
	}
}

var abiSuffixRe = regexp.MustCompile(`<ABI[^>]*>$`)

func stripABISuffix(sym string) string {
	return abiSuffixRe.ReplaceAllString(sym, "")
}

func sigsForAsmFile(pkg *packages.Package, file *plan9asm.File, resolve func(string) string, goarch string) (map[string]plan9asm.FuncSig, error) {
	sigs := map[string]plan9asm.FuncSig{}
	if pkg == nil || pkg.Types == nil || pkg.Types.Scope() == nil {
		for _, fn := range file.Funcs {
			fs := fallbackSigForAsmFunc(fn, resolve(stripABISuffix(fn.Sym)))
			sigs[fs.Name] = fs
		}
		return sigs, nil
	}

	sz := pkg.TypesSizes
	if sz == nil {
		sz = types.SizesFor("gc", goarch)
	}
	if sz == nil {
		return nil, fmt.Errorf("missing type sizes for %q", goarch)
	}

	scope := pkg.Types.Scope()
	linknames := linknameRemoteToLocal(pkg.Syntax)

	for _, fn := range file.Funcs {
		sym := stripABISuffix(fn.Sym)
		resolved := resolve(sym)
		if resolved == "" {
			continue
		}
		fs, ok, err := tryDeclSig(scope, sym, resolved, linknames, goarch, sz)
		if err != nil {
			return nil, err
		}
		if !ok {
			fs = fallbackSigForAsmFunc(fn, resolved)
		}
		sigs[resolved] = fs
	}

	addTargetSig := func(sym string, caller plan9asm.FuncSig, tail bool) {
		if sym == "" {
			return
		}
		sym = stripABISuffix(strings.TrimSuffix(sym, "<>"))
		resolved := resolve(sym)
		if resolved == "" {
			return
		}
		if _, ok := sigs[resolved]; ok {
			return
		}
		fs, ok, err := tryDeclSig(scope, sym, resolved, linknames, goarch, sz)
		if err == nil && ok {
			sigs[resolved] = fs
			return
		}
		if tail && caller.Name != "" {
			copySig := caller
			copySig.Name = resolved
			sigs[resolved] = copySig
			return
		}
		sigs[resolved] = plan9asm.FuncSig{Name: resolved, Ret: plan9asm.I64}
	}

	for _, fn := range file.Funcs {
		caller := sigs[resolve(stripABISuffix(fn.Sym))]
		for _, ins := range fn.Instrs {
			op := strings.ToUpper(string(ins.Op))
			tail := op == "JMP" || op == "B"
			if !(tail || op == "CALL" || op == "BL") {
				continue
			}
			if len(ins.Args) != 1 || ins.Args[0].Kind != plan9asm.OpSym {
				continue
			}
			s := strings.TrimSpace(ins.Args[0].Sym)
			if !strings.HasSuffix(s, "(SB)") {
				continue
			}
			s = strings.TrimSuffix(s, "(SB)")
			base, off := splitSymPlusOff(s)
			if base == "" || off != 0 {
				continue
			}
			addTargetSig(base, caller, tail)
		}
	}

	return sigs, nil
}

func fallbackSigForAsmFunc(fn plan9asm.Func, resolved string) plan9asm.FuncSig {
	paramOff := map[int64]struct{}{}
	retOff := map[int64]struct{}{}

	for _, ins := range fn.Instrs {
		op := strings.ToUpper(string(ins.Op))
		for i, a := range ins.Args {
			if a.Kind != plan9asm.OpFP {
				continue
			}
			if isLikelyResultSlot(op, i, len(ins.Args), a.FPName) {
				retOff[a.FPOffset] = struct{}{}
			} else {
				paramOff[a.FPOffset] = struct{}{}
			}
		}
	}

	paramList := sortOffsets(paramOff)
	retList := sortOffsets(retOff)
	params := make([]plan9asm.FrameSlot, 0, len(paramList))
	for i, off := range paramList {
		params = append(params, plan9asm.FrameSlot{Offset: off, Type: plan9asm.I64, Index: i, Field: -1})
	}
	results := make([]plan9asm.FrameSlot, 0, len(retList))
	for i, off := range retList {
		results = append(results, plan9asm.FrameSlot{Offset: off, Type: plan9asm.I64, Index: i, Field: -1})
	}

	args := make([]plan9asm.LLVMType, len(params))
	for i := range args {
		args[i] = plan9asm.I64
	}
	ret := plan9asm.Void
	switch len(results) {
	case 0:
		ret = plan9asm.I64
	case 1:
		ret = plan9asm.I64
	default:
		parts := make([]string, 0, len(results))
		for range results {
			parts = append(parts, string(plan9asm.I64))
		}
		ret = plan9asm.LLVMType("{ " + strings.Join(parts, ", ") + " }")
	}
	return plan9asm.FuncSig{
		Name: resolved,
		Args: args,
		Ret:  ret,
		Frame: plan9asm.FrameLayout{
			Params:  params,
			Results: results,
		},
	}
}

func isLikelyResultSlot(op string, argIndex int, argCount int, fpName string) bool {
	name := strings.ToLower(fpName)
	if strings.HasPrefix(name, "ret") || strings.HasPrefix(name, "r") && strings.Contains(name, "ret") {
		return true
	}
	if argCount > 0 && argIndex == argCount-1 {
		switch {
		case strings.HasPrefix(op, "MOV"), strings.HasPrefix(op, "VMOV"), strings.HasPrefix(op, "FMOV"):
			return true
		}
	}
	return false
}

func sortOffsets(m map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func tryDeclSig(scope *types.Scope, sym, resolved string, linknames map[string]string, goarch string, sz types.Sizes) (plan9asm.FuncSig, bool, error) {
	declName := strings.TrimPrefix(sym, "·")
	if strings.ContainsRune(declName, '·') {
		key := strings.ReplaceAll(sym, "∕", "/")
		key = strings.ReplaceAll(key, "·", ".")
		if local, ok := linknames[key]; ok {
			declName = local
		} else {
			return plan9asm.FuncSig{}, false, nil
		}
	}
	obj := scope.Lookup(declName)
	if obj == nil {
		return plan9asm.FuncSig{}, false, nil
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return plan9asm.FuncSig{}, false, nil
	}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() != nil || sig.Variadic() {
		return plan9asm.FuncSig{}, false, nil
	}

	args, frameParams, nextOff, err := llvmArgsAndFrameSlotsForTuple(sig.Params(), goarch, sz, 0, false)
	if err != nil {
		return plan9asm.FuncSig{}, false, fmt.Errorf("%s: %w", fn.FullName(), err)
	}
	nextOff = alignOff(nextOff, int64(wordSize(goarch)))
	retTys, frameResults, _, err := llvmArgsAndFrameSlotsForTuple(sig.Results(), goarch, sz, nextOff, true)
	if err != nil {
		return plan9asm.FuncSig{}, false, fmt.Errorf("%s: %w", fn.FullName(), err)
	}
	ret := tupleRetType(retTys)
	return plan9asm.FuncSig{
		Name: resolved,
		Args: args,
		Ret:  ret,
		Frame: plan9asm.FrameLayout{
			Params:  frameParams,
			Results: frameResults,
		},
	}, true, nil
}

func tupleRetType(ts []plan9asm.LLVMType) plan9asm.LLVMType {
	switch len(ts) {
	case 0:
		return plan9asm.Void
	case 1:
		return ts[0]
	default:
		parts := make([]string, 0, len(ts))
		for _, t := range ts {
			parts = append(parts, string(t))
		}
		return plan9asm.LLVMType("{ " + strings.Join(parts, ", ") + " }")
	}
}

func splitSymPlusOff(s string) (base string, off int64) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	sep := strings.LastIndexAny(s, "+-")
	if sep <= 0 || sep == len(s)-1 {
		return s, 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s[sep:]), 0, 64)
	if err != nil {
		return s, 0
	}
	return strings.TrimSpace(s[:sep]), n
}

func linknameRemoteToLocal(files []*ast.File) map[string]string {
	m := map[string]string{}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, cg := range f.Comments {
			if cg == nil {
				continue
			}
			for _, c := range cg.List {
				if c == nil {
					continue
				}
				text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if !strings.HasPrefix(text, "go:linkname ") {
					continue
				}
				parts := strings.Fields(text)
				if len(parts) < 3 {
					continue
				}
				local := parts[1]
				remote := strings.ReplaceAll(parts[2], "∕", "/")
				m[remote] = local
			}
		}
	}
	return m
}

func llvmArgsAndFrameSlotsForTuple(tup *types.Tuple, goarch string, sz types.Sizes, startOff int64, flattenAgg bool) (args []plan9asm.LLVMType, slots []plan9asm.FrameSlot, nextOff int64, err error) {
	if tup == nil || tup.Len() == 0 {
		return nil, nil, startOff, nil
	}

	word := int64(wordSize(goarch))
	wordTy := plan9asm.I64
	if word == 4 {
		wordTy = plan9asm.LLVMType("i32")
	}
	align := func(off, a int64) int64 {
		if a <= 1 {
			return off
		}
		m := off % a
		if m == 0 {
			return off
		}
		return off + (a - m)
	}

	off := startOff
	argIdx := 0
	for i := 0; i < tup.Len(); i++ {
		t := tup.At(i).Type()
		off = align(off, int64(sz.Alignof(t)))

		switch u := types.Unalias(t).(type) {
		case *types.Basic:
			if u.Kind() == types.String {
				if flattenAgg {
					args = append(args, plan9asm.Ptr, wordTy)
					slots = append(slots,
						plan9asm.FrameSlot{Offset: off + 0*word, Type: plan9asm.Ptr, Index: argIdx + 0, Field: -1},
						plan9asm.FrameSlot{Offset: off + 1*word, Type: wordTy, Index: argIdx + 1, Field: -1},
					)
					argIdx += 2
					off += int64(sz.Sizeof(t))
					continue
				}
				ty, e := llvmTypeForGo(t, goarch)
				if e != nil {
					return nil, nil, 0, e
				}
				args = append(args, ty)
				slots = append(slots,
					plan9asm.FrameSlot{Offset: off + 0*word, Type: plan9asm.Ptr, Index: argIdx, Field: 0},
					plan9asm.FrameSlot{Offset: off + 1*word, Type: wordTy, Index: argIdx, Field: 1},
				)
				argIdx++
				off += int64(sz.Sizeof(t))
				continue
			}
		case *types.Slice:
			if flattenAgg {
				args = append(args, plan9asm.Ptr, wordTy, wordTy)
				slots = append(slots,
					plan9asm.FrameSlot{Offset: off + 0*word, Type: plan9asm.Ptr, Index: argIdx + 0, Field: -1},
					plan9asm.FrameSlot{Offset: off + 1*word, Type: wordTy, Index: argIdx + 1, Field: -1},
					plan9asm.FrameSlot{Offset: off + 2*word, Type: wordTy, Index: argIdx + 2, Field: -1},
				)
				argIdx += 3
				off += int64(sz.Sizeof(t))
				continue
			}
			ty, e := llvmTypeForGo(t, goarch)
			if e != nil {
				return nil, nil, 0, e
			}
			args = append(args, ty)
			slots = append(slots,
				plan9asm.FrameSlot{Offset: off + 0*word, Type: plan9asm.Ptr, Index: argIdx, Field: 0},
				plan9asm.FrameSlot{Offset: off + 1*word, Type: wordTy, Index: argIdx, Field: 1},
				plan9asm.FrameSlot{Offset: off + 2*word, Type: wordTy, Index: argIdx, Field: 2},
			)
			argIdx++
			off += int64(sz.Sizeof(t))
			continue
		}

		ty, e := llvmTypeForGo(t, goarch)
		if e != nil {
			return nil, nil, 0, e
		}
		args = append(args, ty)
		slots = append(slots, plan9asm.FrameSlot{Offset: off, Type: ty, Index: argIdx, Field: -1})
		argIdx++
		off += int64(sz.Sizeof(t))
	}
	return args, slots, off, nil
}

func llvmTypeForGo(t types.Type, goarch string) (plan9asm.LLVMType, error) {
	switch tt := t.(type) {
	case *types.Basic:
		switch tt.Kind() {
		case types.Bool:
			return plan9asm.I1, nil
		case types.UnsafePointer:
			return plan9asm.Ptr, nil
		case types.Int8, types.Uint8:
			return plan9asm.I8, nil
		case types.Int16, types.Uint16:
			return plan9asm.I16, nil
		case types.Int32, types.Uint32:
			return plan9asm.I32, nil
		case types.Int64, types.Uint64:
			return plan9asm.I64, nil
		case types.Int, types.Uint, types.Uintptr:
			if wordSize(goarch) == 8 {
				return plan9asm.I64, nil
			}
			return plan9asm.I32, nil
		case types.Float32:
			return plan9asm.LLVMType("float"), nil
		case types.Float64:
			return plan9asm.LLVMType("double"), nil
		case types.String:
			if wordSize(goarch) == 8 {
				return plan9asm.LLVMType("{ ptr, i64 }"), nil
			}
			return plan9asm.LLVMType("{ ptr, i32 }"), nil
		default:
			return "", fmt.Errorf("unsupported basic type %s", tt.String())
		}
	case *types.Pointer:
		return plan9asm.Ptr, nil
	case *types.Slice:
		if wordSize(goarch) == 8 {
			return plan9asm.LLVMType("{ ptr, i64, i64 }"), nil
		}
		return plan9asm.LLVMType("{ ptr, i32, i32 }"), nil
	case *types.Named:
		return llvmTypeForGo(tt.Underlying(), goarch)
	default:
		return "", fmt.Errorf("unsupported type %s", t.String())
	}
}

func wordSize(goarch string) int {
	switch goarch {
	case "amd64", "arm64", "loong64", "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x":
		return 8
	default:
		return 4
	}
}

func alignOff(off, a int64) int64 {
	if a <= 1 {
		return off
	}
	m := off % a
	if m == 0 {
		return off
	}
	return off + (a - m)
}

func check(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
