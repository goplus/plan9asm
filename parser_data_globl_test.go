package plan9asm

import "testing"

func TestParseDataAndGloblDirectives(t *testing.T) {
	file, err := Parse(ArchARM64, `TEXT ·Fn(SB),NOSPLIT,$0-0
	RET

DATA ·tab<>+8(SB)/PTRSIZE, $1
DATA ·str<>(SB)/8, $"hello"
DATA ·symptr<>(SB)/8, $runtime·main(SB)
GLOBL ·tab<>(SB), RODATA, $16
GLOBL ·symptr<>(SB), NOPTR, $(machTimebaseInfo__size)
`)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(file.Data), 3; got != want {
		t.Fatalf("len(Data) = %d, want %d", got, want)
	}
	if got, want := len(file.Globl), 2; got != want {
		t.Fatalf("len(Globl) = %d, want %d", got, want)
	}

	if ds := file.Data[0]; ds.Sym != "·tab<>" || ds.Off != 8 || ds.Width != 8 || ds.Value != 1 {
		t.Fatalf("unexpected first DATA: %#v", ds)
	}
	if ds := file.Data[1]; ds.Sym != "·str<>" || ds.Value != 0 {
		t.Fatalf("unexpected string DATA placeholder: %#v", ds)
	}
	if ds := file.Data[2]; ds.Sym != "·symptr<>" || ds.Value != 0 {
		t.Fatalf("unexpected symbol DATA placeholder: %#v", ds)
	}

	if gs := file.Globl[0]; gs.Sym != "·tab<>" || gs.Flags != "RODATA" || gs.Size != 16 {
		t.Fatalf("unexpected first GLOBL: %#v", gs)
	}
	if gs := file.Globl[1]; gs.Sym != "·symptr<>" || gs.Flags != "NOPTR" || gs.Size != 64 {
		t.Fatalf("unexpected macro-sized GLOBL: %#v", gs)
	}
}
