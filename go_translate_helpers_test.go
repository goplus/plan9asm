package plan9asm

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestGoHelperDeclNameAndLinkname(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "pkg.go", `package p
//go:linkname cmp runtime.cmp
func cmp()
`, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	linknames := goLinknameRemoteToLocal([]*ast.File{f})
	if got := linknames["runtime.cmp"]; got != "cmp" {
		t.Fatalf("runtime.cmp => %q, want cmp", got)
	}

	got, err := goDeclNameForSymbol("·Compare", linknames)
	if err != nil || got != "Compare" {
		t.Fatalf("goDeclNameForSymbol local = (%q, %v), want Compare", got, err)
	}

	got, err = goDeclNameForSymbol("runtime·cmp", linknames)
	if err != nil || got != "cmp" {
		t.Fatalf("goDeclNameForSymbol linkname = (%q, %v), want cmp", got, err)
	}

	if _, err := goDeclNameForSymbol("runtime·missing", linknames); err == nil {
		t.Fatalf("expected missing go:linkname mapping error")
	}
}

func TestGoHelperArchTupleAndSymParsing(t *testing.T) {
	if got, err := goArchFor("amd64"); err != nil || got != ArchAMD64 {
		t.Fatalf("goArchFor amd64 = (%q, %v), want %q", got, err, ArchAMD64)
	}
	if got, err := goArchFor("arm64"); err != nil || got != ArchARM64 {
		t.Fatalf("goArchFor arm64 = (%q, %v), want %q", got, err, ArchARM64)
	}
	if _, err := goArchFor("wasm"); err == nil {
		t.Fatalf("expected unsupported arch error")
	}

	if got := goTupleRetType(nil); got != Void {
		t.Fatalf("goTupleRetType(nil) = %q, want %q", got, Void)
	}
	if got := goTupleRetType([]LLVMType{I64}); got != I64 {
		t.Fatalf("goTupleRetType(single) = %q, want %q", got, I64)
	}
	if got := goTupleRetType([]LLVMType{I64, Ptr}); got != "{ i64, ptr }" {
		t.Fatalf("goTupleRetType(tuple) = %q", got)
	}

	base, off := goSplitSymPlusOff("runtime·foo+16")
	if base != "runtime·foo" || off != 16 {
		t.Fatalf("goSplitSymPlusOff(+16) = (%q, %d)", base, off)
	}
	base, off = goSplitSymPlusOff("runtime·foo-8")
	if base != "runtime·foo" || off != -8 {
		t.Fatalf("goSplitSymPlusOff(-8) = (%q, %d)", base, off)
	}

	base, tail, ok := goReferencedFunc(Instr{
		Op: "JMP",
		Args: []Operand{{
			Kind: OpSym,
			Sym:  "indexbody<>(SB)",
		}},
	})
	if !ok || !tail || base != "indexbody<>" {
		t.Fatalf("goReferencedFunc tail jump = (%q, %v, %v)", base, tail, ok)
	}

	base, tail, ok = goReferencedFunc(Instr{
		Op: "CALL",
		Args: []Operand{{
			Kind: OpSym,
			Sym:  "runtime·cmp(SB)",
		}},
	})
	if !ok || tail || base != "runtime·cmp" {
		t.Fatalf("goReferencedFunc call = (%q, %v, %v)", base, tail, ok)
	}
}

func TestGoExpandConsts(t *testing.T) {
	pkg := types.NewPackage("test/pkg", "pkg")
	addIntConst(pkg, "Local", 7)

	runtimePkg := types.NewPackage("runtime", "runtime")
	addIntConst(runtimePkg, "Const", 3)

	src := []byte(`MOVD $const_Local, R0
MOVD foo+const_Local(SB), R1
MOVD runtime.foo+const_Const(SB), R2
MOVD runtime/foo+const_Const(SB), R3
MOVD missing+const_Missing(SB), R4
`)
	got := string(goExpandConsts(src, pkg, map[string]*types.Package{
		"runtime": runtimePkg,
	}))

	for _, want := range []string{
		"MOVD $7, R0",
		"MOVD foo+7(SB), R1",
		"MOVD runtime.foo+3(SB), R2",
		"MOVD runtime/foo+3(SB), R3",
		"MOVD missing+const_Missing(SB), R4",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expanded consts missing %q in:\n%s", want, got)
		}
	}
}

func TestGoLLVMHelpers(t *testing.T) {
	iface := types.NewInterfaceType(nil, nil)
	iface.Complete()
	namedSlice := types.NewNamed(
		types.NewTypeName(token.NoPos, nil, "Bytes", nil),
		types.NewSlice(types.Typ[types.Byte]),
		nil,
	)

	cases := []struct {
		name string
		typ  types.Type
		want LLVMType
	}{
		{name: "bool", typ: types.Typ[types.Bool], want: I1},
		{name: "uintptr", typ: types.Typ[types.Uintptr], want: I64},
		{name: "string", typ: types.Typ[types.String], want: "{ ptr, i64 }"},
		{name: "slice", typ: types.NewSlice(types.Typ[types.Byte]), want: "{ ptr, i64, i64 }"},
		{name: "interface", typ: iface, want: "{ ptr, ptr }"},
		{name: "named", typ: namedSlice, want: "{ ptr, i64, i64 }"},
	}
	for _, tc := range cases {
		got, err := goLLVMTypeForType(tc.typ, "arm64")
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}

	if _, err := goLLVMTypeForType(types.NewMap(types.Typ[types.String], types.Typ[types.Int]), "arm64"); err == nil {
		t.Fatalf("expected unsupported type error")
	}

	sz := types.SizesFor("gc", "arm64")
	tup := types.NewTuple(
		types.NewVar(token.NoPos, nil, "s", types.Typ[types.String]),
		types.NewVar(token.NoPos, nil, "b", types.NewSlice(types.Typ[types.Byte])),
		types.NewVar(token.NoPos, nil, "v", iface),
	)

	args, slots, next, err := goLLVMArgsAndFrameSlotsForTuple(tup, "arm64", sz, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("flatten=false args len = %d, want %d", got, want)
	}
	if got, want := len(slots), 7; got != want {
		t.Fatalf("flatten=false slots len = %d, want %d", got, want)
	}
	if next != 56 {
		t.Fatalf("flatten=false next = %d, want 56", next)
	}
	if slots[0].Field != 0 || slots[1].Field != 1 || slots[2].Field != 0 {
		t.Fatalf("unexpected slot field mapping: %#v", slots[:3])
	}

	args, slots, next, err = goLLVMArgsAndFrameSlotsForTuple(tup, "arm64", sz, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(args), 7; got != want {
		t.Fatalf("flatten=true args len = %d, want %d", got, want)
	}
	if got, want := len(slots), 7; got != want {
		t.Fatalf("flatten=true slots len = %d, want %d", got, want)
	}
	if next != 56 {
		t.Fatalf("flatten=true next = %d, want 56", next)
	}
}

func TestGoFuncSigForDeclaredFuncRejectsUnsupportedForms(t *testing.T) {
	sz := types.SizesFor("gc", "arm64")
	recvType := types.NewNamed(types.NewTypeName(token.NoPos, nil, "Receiver", nil), types.NewStruct(nil, nil), nil)
	recv := types.NewVar(token.NoPos, nil, "r", recvType)

	methodSig := types.NewSignature(recv, types.NewTuple(), types.NewTuple(), false)
	method := types.NewFunc(token.NoPos, nil, "Method", methodSig)
	if _, err := goFuncSigForDeclaredFunc("pkg.Method", method, "arm64", sz, true); err == nil || !strings.Contains(err.Error(), "methods in asm not supported") {
		t.Fatalf("expected method rejection, got %v", err)
	}

	variadicSig := types.NewSignature(
		nil,
		types.NewTuple(types.NewVar(token.NoPos, nil, "args", types.NewSlice(types.Typ[types.Int]))),
		types.NewTuple(),
		true,
	)
	variadic := types.NewFunc(token.NoPos, nil, "Variadic", variadicSig)
	if _, err := goFuncSigForDeclaredFunc("pkg.Variadic", variadic, "arm64", sz, true); err == nil || !strings.Contains(err.Error(), "variadic asm not supported") {
		t.Fatalf("expected variadic rejection, got %v", err)
	}
}

func addIntConst(pkg *types.Package, name string, v int64) {
	pkg.Scope().Insert(types.NewConst(token.NoPos, pkg, name, types.Typ[types.Int], constant.MakeInt64(v)))
}
