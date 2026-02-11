# plan9asm

`github.com/goplus/plan9asm`

Plan 9 assembly parser and LLVM IR translator, extracted as an independent module.

## Highlights

- Parse a practical subset of Go/Plan 9 asm (`amd64`, `arm64`).
- Translate asm to LLVM IR with configurable symbol/signature mapping.
- Compile-time checks with `llc` for multiple target triples.
- Runtime validation tests (`ll` -> `llc` -> `clang` -> run) for selected branch/flag behaviors.

## Test

```bash
go test ./...
```

Some tests require local LLVM/Clang tools (`llc`, `clang`) and may skip when unavailable.
