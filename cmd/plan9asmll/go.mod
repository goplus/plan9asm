module github.com/goplus/plan9asm/cmd/plan9asmll

go 1.24.0

require (
	github.com/goplus/llvm v0.8.6
	github.com/goplus/plan9asm v0.0.0
	golang.org/x/tools v0.42.0
)

require (
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)

replace github.com/goplus/plan9asm => ../..
