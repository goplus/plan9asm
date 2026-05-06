package plan9asm

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/xgo-dev/llvm"
)

// GoPackage provides the Go declarations and syntax needed to bind a Plan 9
// asm file to real Go symbols.
type GoPackage struct {
	Path    string
	Types   *types.Package
	Imports map[string]*types.Package
	Syntax  []*ast.File
}

// GoModuleOptions configures TranslateGoModule.
//
// GOARCH is required and currently accepts only "amd64", "386", and "arm64".
// If ResolveSym is nil, the default resolver only strips ABI suffixes.
type GoModuleOptions struct {
	FileName       string
	GOOS           string
	GOARCH         string
	TargetTriple   string
	AnnotateSource bool

	ResolveSym func(sym string) string
	KeepFunc   func(textSym, resolved string) bool
	ManualSig  func(resolved string) (FuncSig, bool)
}

// GoFunction records the original TEXT symbol and its resolved LLVM symbol.
type GoFunction struct {
	TextSymbol     string
	ResolvedSymbol string
}

// GoModuleTranslation is the result of TranslateGoModule.
//
// Callers own Module and must call Module.Dispose when finished with it.
type GoModuleTranslation struct {
	Module     llvm.Module
	Signatures map[string]FuncSig
	Functions  []GoFunction
}

// TranslateGoModule binds Go declarations to a Plan 9 asm file and translates
// the result into an LLVM module in one call.
//
// The package must provide go/types information for the declarations referenced
// by the assembly. Methods and variadic functions are not supported.
func TranslateGoModule(pkg GoPackage, src []byte, opt GoModuleOptions) (*GoModuleTranslation, error) {
	pkgPath := pkg.Path
	if pkgPath == "" && pkg.Types != nil {
		pkgPath = pkg.Types.Path()
	}
	if pkgPath == "" {
		return nil, fmt.Errorf("empty package path")
	}
	if pkg.Types == nil || pkg.Types.Scope() == nil {
		return nil, fmt.Errorf("%s: missing types (needed for asm signatures)", pkgPath)
	}
	if opt.GOARCH == "" {
		return nil, fmt.Errorf("%s: empty GOARCH", pkgPath)
	}
	asmName := opt.FileName
	if asmName == "" {
		asmName = "<asm>"
	}

	arch, err := goArchFor(opt.GOARCH)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pkgPath, err)
	}
	resolve := opt.ResolveSym
	if resolve == nil {
		resolve = func(sym string) string { return goStripABISuffix(sym) }
	}

	if bytes.Contains(src, []byte("const_")) {
		src = goExpandConsts(src, pkg.Types, pkg.Imports)
	}

	file, err := Parse(arch, string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: parse %s: %w", pkgPath, asmName, err)
	}
	if opt.KeepFunc != nil {
		keep := make([]Func, 0, len(file.Funcs))
		for _, fn := range file.Funcs {
			resolved := resolve(goStripABISuffix(fn.Sym))
			if opt.KeepFunc(fn.Sym, resolved) {
				keep = append(keep, fn)
			}
		}
		file.Funcs = keep
	}

	sigs, err := goSigsForAsmFile(pkg, file, resolve, opt.GOARCH, opt.ManualSig)
	if err != nil {
		return nil, fmt.Errorf("%s: sigs %s: %w", pkgPath, asmName, err)
	}
	mod, err := TranslateModule(file, Options{
		TargetTriple:   opt.TargetTriple,
		ResolveSym:     resolve,
		Sigs:           sigs,
		Goarch:         opt.GOARCH,
		AnnotateSource: opt.AnnotateSource,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: translate %s: %w", pkgPath, asmName, err)
	}

	funcs := make([]GoFunction, 0, len(file.Funcs))
	for _, fn := range file.Funcs {
		sym := goStripABISuffix(fn.Sym)
		funcs = append(funcs, GoFunction{TextSymbol: fn.Sym, ResolvedSymbol: resolve(sym)})
	}

	return &GoModuleTranslation{Module: mod, Signatures: sigs, Functions: funcs}, nil
}

var goABISuffixRe = regexp.MustCompile(`<ABI[^>]*>$`)
var goConstRefRe = regexp.MustCompile(`\bconst_[A-Za-z0-9_]+\b`)
var goConstPlusRefRe = regexp.MustCompile(`([\pL\pN_∕·./]+)\+const_([A-Za-z0-9_]+)`)

func goStripABISuffix(sym string) string {
	sym = goABISuffixRe.ReplaceAllString(sym, "")
	return strings.TrimSuffix(sym, "<>")
}

func goArchFor(goarch string) (Arch, error) {
	switch goarch {
	case "amd64", "386":
		return ArchAMD64, nil
	case "arm":
		return ArchARM, nil
	case "arm64":
		return ArchARM64, nil
	default:
		return "", fmt.Errorf("Plan 9 asm unsupported arch %q", goarch)
	}
}

func goSigsForAsmFile(pkg GoPackage, file *File, resolve func(sym string) string, goarch string, manualSig func(string) (FuncSig, bool)) (map[string]FuncSig, error) {
	sz := types.SizesFor("gc", goarch)
	if sz == nil {
		return nil, fmt.Errorf("missing sizes for goarch %q", goarch)
	}
	b := goSigBuilder{
		sigs:      make(map[string]FuncSig, len(file.Funcs)),
		scope:     pkg.Types.Scope(),
		sz:        sz,
		linknames: goLinknameRemoteToLocal(pkg.Syntax),
		pkgPath:   pkg.Path,
		resolve:   resolve,
		goarch:    goarch,
		manualSig: manualSig,
	}
	if b.pkgPath == "" {
		b.pkgPath = pkg.Types.Path()
	}
	if err := b.addDeclaredFuncSigs(file); err != nil {
		return nil, err
	}
	if err := b.addReferencedFuncSigs(file); err != nil {
		return nil, err
	}
	return b.sigs, nil
}

type goSigBuilder struct {
	sigs      map[string]FuncSig
	scope     *types.Scope
	sz        types.Sizes
	linknames map[string]string
	pkgPath   string
	resolve   func(sym string) string
	goarch    string
	manualSig func(string) (FuncSig, bool)
}

func (b *goSigBuilder) addDeclaredFuncSigs(file *File) error {
	for i := range file.Funcs {
		sym := goStripABISuffix(file.Funcs[i].Sym)
		resolved := b.resolve(sym)
		if ms, ok := goLookupManualSig(b.manualSig, resolved); ok {
			b.sigs[resolved] = ms
			continue
		}

		declName, err := goDeclNameForSymbol(sym, b.linknames)
		if err != nil {
			return err
		}
		obj := b.scope.Lookup(declName)
		if obj == nil {
			return fmt.Errorf("missing Go declaration for asm symbol %q", sym)
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			return fmt.Errorf("asm symbol %q maps to non-func %T", sym, obj)
		}
		fs, err := goFuncSigForDeclaredFunc(resolved, fn, b.goarch, b.sz, true)
		if err != nil {
			return err
		}
		b.sigs[resolved] = fs
	}
	return nil
}

func (b *goSigBuilder) addReferencedFuncSigs(file *File) error {
	for _, fn := range file.Funcs {
		callerResolved := b.resolve(goStripABISuffix(fn.Sym))
		callerSig, hasCallerSig := b.sigs[callerResolved]
		for _, ins := range fn.Instrs {
			base, tailJump, ok := goReferencedFunc(ins)
			if !ok {
				continue
			}
			if err := b.addGoDeclSig(base); err != nil {
				return err
			}
			targetResolved := b.resolve(base)
			if _, ok := b.sigs[targetResolved]; ok {
				continue
			}
			if !tailJump || !hasCallerSig {
				continue
			}
			// Best-effort fallback for helper<> tail-jumps where the callee is an
			// internal asm label with no Go declaration. Callers can override this
			// via ManualSig when the inferred signature is not identical.
			fs := callerSig
			fs.Name = targetResolved
			b.sigs[targetResolved] = fs
		}
	}
	return nil
}

func (b *goSigBuilder) addGoDeclSig(sym string) error {
	sym = goStripABISuffix(sym)
	resolved := b.resolve(sym)
	if resolved == "" {
		return nil
	}
	if _, ok := b.sigs[resolved]; ok {
		return nil
	}
	if ms, ok := goLookupManualSig(b.manualSig, resolved); ok {
		b.sigs[resolved] = ms
		return nil
	}

	declName := ""
	remoteKey := strings.ReplaceAll(sym, "∕", "/")
	remoteKey = strings.ReplaceAll(remoteKey, "·", ".")
	if local, ok := b.linknames[remoteKey]; ok {
		declName = local
	} else if strings.HasPrefix(resolved, b.pkgPath+".") {
		declName = strings.TrimPrefix(resolved, b.pkgPath+".")
	} else if strings.HasPrefix(sym, "·") {
		declName = strings.TrimPrefix(sym, "·")
	}
	if declName == "" {
		return nil
	}

	obj := b.scope.Lookup(declName)
	if obj == nil {
		return nil
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	fs, err := goFuncSigForDeclaredFunc(resolved, fn, b.goarch, b.sz, false)
	if err != nil {
		return err
	}
	b.sigs[resolved] = fs
	return nil
}

func goReferencedFunc(ins Instr) (base string, tailJump bool, ok bool) {
	switch string(ins.Op) {
	case "JMP", "B":
		tailJump = true
	case "CALL", "BL":
	default:
		return "", false, false
	}
	if len(ins.Args) != 1 || ins.Args[0].Kind != OpSym {
		return "", false, false
	}
	s := strings.TrimSpace(ins.Args[0].Sym)
	if !strings.HasSuffix(s, "(SB)") {
		return "", false, false
	}
	s = strings.TrimSuffix(s, "(SB)")
	base, off := goSplitSymPlusOff(s)
	if base == "" || off != 0 {
		return "", false, false
	}
	return base, tailJump, true
}

func goSplitSymPlusOff(s string) (base string, off int64) {
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

func goLookupManualSig(manual func(string) (FuncSig, bool), resolved string) (FuncSig, bool) {
	if manual == nil {
		return FuncSig{}, false
	}
	fs, ok := manual(resolved)
	if !ok {
		return FuncSig{}, false
	}
	if fs.Name == "" {
		fs.Name = resolved
	}
	return fs, true
}

func goDeclNameForSymbol(sym string, linknames map[string]string) (string, error) {
	declName := strings.TrimPrefix(sym, "·")
	if strings.ContainsRune(declName, '·') {
		key := strings.ReplaceAll(sym, "∕", "/")
		key = strings.ReplaceAll(key, "·", ".")
		if local, ok := linknames[key]; ok {
			return local, nil
		}
		return "", fmt.Errorf("unsupported asm symbol name %q (no go:linkname mapping found)", sym)
	}
	return declName, nil
}

func goFuncSigForDeclaredFunc(name string, fn *types.Func, goarch string, sz types.Sizes, withFrame bool) (FuncSig, error) {
	sig := fn.Type().(*types.Signature)
	if sig.Recv() != nil {
		return FuncSig{}, fmt.Errorf("methods in asm not supported: %s", fn.FullName())
	}
	if sig.Variadic() {
		return FuncSig{}, fmt.Errorf("variadic asm not supported: %s", fn.FullName())
	}
	if withFrame {
		args, frameParams, nextOff, err := goLLVMArgsAndFrameSlotsForTuple(sig.Params(), goarch, sz, 0, false)
		if err != nil {
			return FuncSig{}, fmt.Errorf("%s: %w", fn.FullName(), err)
		}
		nextOff = goAlignOff(nextOff, int64(goWordSize(goarch)))
		retTys, frameResults, _, err := goLLVMArgsAndFrameSlotsForTuple(sig.Results(), goarch, sz, nextOff, true)
		if err != nil {
			return FuncSig{}, fmt.Errorf("%s: %w", fn.FullName(), err)
		}
		return FuncSig{Name: name, Args: args, Ret: goTupleRetType(retTys), Frame: FrameLayout{Params: frameParams, Results: frameResults}}, nil
	}
	args, _, _, err := goLLVMArgsAndFrameSlotsForTuple(sig.Params(), goarch, sz, 0, false)
	if err != nil {
		return FuncSig{}, fmt.Errorf("%s: %w", fn.FullName(), err)
	}
	retTys, _, _, err := goLLVMArgsAndFrameSlotsForTuple(sig.Results(), goarch, sz, 0, false)
	if err != nil {
		return FuncSig{}, fmt.Errorf("%s: %w", fn.FullName(), err)
	}
	return FuncSig{Name: name, Args: args, Ret: goTupleRetType(retTys)}, nil
}

func goTupleRetType(ts []LLVMType) LLVMType {
	switch len(ts) {
	case 0:
		return Void
	case 1:
		return ts[0]
	default:
		parts := make([]string, 0, len(ts))
		for _, t := range ts {
			parts = append(parts, string(t))
		}
		return LLVMType("{ " + strings.Join(parts, ", ") + " }")
	}
}

func goLinknameRemoteToLocal(files []*ast.File) map[string]string {
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
				line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if !strings.HasPrefix(line, "go:linkname") {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) < 3 || parts[0] != "go:linkname" {
					continue
				}
				local := parts[1]
				remote := strings.ReplaceAll(parts[2], "∕", "/")
				remote = strings.ReplaceAll(remote, "·", ".")
				m[remote] = local
			}
		}
	}
	return m
}

func goExpandConsts(src []byte, pkgTypes *types.Package, imports map[string]*types.Package) []byte {
	if pkgTypes == nil || pkgTypes.Scope() == nil {
		return src
	}
	typeByPath := map[string]*types.Package{pkgTypes.Path(): pkgTypes}
	for path, tp := range imports {
		if tp != nil && tp.Scope() != nil && typeByPath[path] == nil {
			typeByPath[path] = tp
		}
	}
	lookupConst := func(tp *types.Package, name string) (string, bool) {
		if tp == nil || tp.Scope() == nil || name == "" {
			return "", false
		}
		obj := tp.Scope().Lookup(name)
		c, ok := obj.(*types.Const)
		if !ok || c == nil || c.Val() == nil {
			return "", false
		}
		if i64, ok := constant.Int64Val(c.Val()); ok {
			return fmt.Sprintf("%d", i64), true
		}
		return "", false
	}

	src = goConstPlusRefRe.ReplaceAllFunc(src, func(m []byte) []byte {
		sub := goConstPlusRefRe.FindSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		prefix := string(sub[1])
		name := string(sub[2])
		if i := strings.LastIndex(prefix, "/"); i >= 0 {
			path := prefix[:i]
			sym := prefix[i+1:]
			if tp := typeByPath[path]; tp != nil {
				if val, ok := lookupConst(tp, name); ok {
					return []byte(path + "/" + sym + "+" + val)
				}
			}
		}
		if j := strings.LastIndex(prefix, "."); j >= 0 {
			path := prefix[:j]
			sym := prefix[j+1:]
			if tp := typeByPath[path]; tp != nil {
				if val, ok := lookupConst(tp, name); ok {
					return []byte(path + "." + sym + "+" + val)
				}
			}
		}
		if val, ok := lookupConst(pkgTypes, name); ok {
			return []byte(prefix + "+" + val)
		}
		return m
	})

	return goConstRefRe.ReplaceAllFunc(src, func(m []byte) []byte {
		name := strings.TrimPrefix(string(m), "const_")
		if val, ok := lookupConst(pkgTypes, name); ok {
			return []byte(val)
		}
		return m
	})
}

func goLLVMTypeForType(t types.Type, goarch string) (LLVMType, error) {
	switch tt := t.(type) {
	case *types.Basic:
		switch tt.Kind() {
		case types.Bool:
			return I1, nil
		case types.UnsafePointer:
			return Ptr, nil
		case types.Int8, types.Uint8:
			return LLVMType("i8"), nil
		case types.Int16, types.Uint16:
			return LLVMType("i16"), nil
		case types.Int32, types.Uint32:
			return I32, nil
		case types.Int64, types.Uint64:
			return I64, nil
		case types.Int, types.Uint, types.Uintptr:
			if goWordSize(goarch) == 8 {
				return I64, nil
			}
			return I32, nil
		case types.Float32:
			return LLVMType("float"), nil
		case types.Float64:
			return LLVMType("double"), nil
		case types.String:
			if goWordSize(goarch) == 8 {
				return LLVMType("{ ptr, i64 }"), nil
			}
			return LLVMType("{ ptr, i32 }"), nil
		default:
			return "", fmt.Errorf("unsupported basic type %s", tt.String())
		}
	case *types.Pointer:
		return Ptr, nil
	case *types.Slice:
		if goWordSize(goarch) == 8 {
			return LLVMType("{ ptr, i64, i64 }"), nil
		}
		return LLVMType("{ ptr, i32, i32 }"), nil
	case *types.Interface:
		return LLVMType("{ ptr, ptr }"), nil
	case *types.Named:
		return goLLVMTypeForType(tt.Underlying(), goarch)
	default:
		return "", fmt.Errorf("unsupported type %s", t.String())
	}
}

func goLLVMArgsAndFrameSlotsForTuple(tup *types.Tuple, goarch string, sz types.Sizes, startOff int64, flattenAgg bool) (args []LLVMType, slots []FrameSlot, nextOff int64, err error) {
	if tup == nil || tup.Len() == 0 {
		return nil, nil, startOff, nil
	}

	off := startOff
	argIdx := 0
	for i := 0; i < tup.Len(); i++ {
		t := tup.At(i).Type()
		a := int64(sz.Alignof(t))
		off = goAlignOff(off, a)

		parts, ok := goFramePartsForType(t, goarch)
		if ok {
			if flattenAgg {
				for _, part := range parts {
					args = append(args, part.Type)
					slots = append(slots, FrameSlot{Offset: off + part.Offset, Type: part.Type, Index: argIdx, Field: -1})
					argIdx++
				}
			} else {
				ty, e := goLLVMTypeForType(t, goarch)
				if e != nil {
					return nil, nil, 0, e
				}
				args = append(args, ty)
				for _, part := range parts {
					slots = append(slots, FrameSlot{Offset: off + part.Offset, Type: part.Type, Index: argIdx, Field: part.Field})
				}
				argIdx++
			}
			off += int64(sz.Sizeof(t))
			continue
		}

		ty, e := goLLVMTypeForType(t, goarch)
		if e != nil {
			return nil, nil, 0, e
		}
		args = append(args, ty)
		slots = append(slots, FrameSlot{Offset: off, Type: ty, Index: argIdx, Field: -1})
		argIdx++
		off += int64(sz.Sizeof(t))
	}
	return args, slots, off, nil
}

type goFramePart struct {
	Offset int64
	Type   LLVMType
	Field  int
}

func goFramePartsForType(t types.Type, goarch string) ([]goFramePart, bool) {
	word := int64(goWordSize(goarch))
	wordTy := I64
	if word == 4 {
		wordTy = I32
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		if u.Kind() == types.String {
			return []goFramePart{{Offset: 0, Type: Ptr, Field: 0}, {Offset: word, Type: wordTy, Field: 1}}, true
		}
	case *types.Slice:
		return []goFramePart{{Offset: 0, Type: Ptr, Field: 0}, {Offset: word, Type: wordTy, Field: 1}, {Offset: 2 * word, Type: wordTy, Field: 2}}, true
	case *types.Interface:
		return []goFramePart{{Offset: 0, Type: Ptr, Field: 0}, {Offset: word, Type: Ptr, Field: 1}}, true
	}
	return nil, false
}

func goWordSize(goarch string) int {
	switch goarch {
	case "amd64", "arm64":
		return 8
	default:
		return 4
	}
}

func goAlignOff(off, a int64) int64 {
	if a <= 1 {
		return off
	}
	m := off % a
	if m == 0 {
		return off
	}
	return off + (a - m)
}
