module github.com/goplus/plan9asm/cmd/plan9asm

go 1.21

require (
	github.com/goplus/plan9asm v0.0.0
	golang.org/x/tools v0.24.0
)

require (
	github.com/goplus/llvm v0.8.5 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
)

replace github.com/goplus/plan9asm => ../..
