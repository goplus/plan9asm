# plan9asm LLVM Module Migration (goplus/llvm)

## Goal

Current `plan9asm.Translate` emits textual `.ll` by string concatenation.
Target architecture:

1. Build an in-memory `llvm.Module` (via `github.com/goplus/llvm`).
2. Caller can choose:
   - get module handle directly
   - serialize module to string/bitcode as needed

## Proposed API

Keep backward compatibility first:

```go
// Existing API (keep)
func Translate(file *File, opt Options) (string, error)

// New API
func TranslateModule(file *File, opt Options) (llvm.Module, error)
```

Then implement `Translate` as:

```go
func Translate(file *File, opt Options) (string, error) {
    m, err := TranslateModule(file, opt)
    if err != nil { return "", err }
    return moduleToIRString(m), nil
}
```

## Migration Plan

### Phase 1: IR abstraction layer

- Introduce internal emitter interface (`irEmitter`) with operations used by current lowerers
- Keep current string backend as default implementation
- No behavior change

### Phase 2: llvm backend

- Add `llvmEmitter` implementation backed by `goplus/llvm` builders
- Start from function skeleton + basic blocks + integer ops
- Keep unsupported ops behavior identical

### Phase 3: switch entrypoint

- `TranslateModule` uses `llvmEmitter`
- `Translate` calls `TranslateModule` then serializes
- Keep golden tests for textual IR stability where needed

## Risks

- SSA temp naming and instruction order differences can change textual IR
- Attribute/intrinsic emission must remain target-correct
- Module lifetime management (`Context`, `Module`, dispose)

## Validation

- Reuse existing test corpus in this repo
- Add equivalence checks:
  - object build with `llc`
  - selected runtime execution tests
