# plan9asm -> llgo/cabi 签名降级设计（Slice 与聚合参数）

## 背景

在 llgo 的 plan9 asm 翻译链路中，`name+off(FP)` 需要映射到 LLVM 参数/返回值。
历史实现对 `slice`/`string` 做了固定头部展开（例如 slice 固定 3 word），这会导致规则分散、扩展困难。

目标是把“如何展开”绑定到 Go 声明类型本身（declaration-driven），并和 cabi 的后续 ABI 转换配合。

## 目标

1. 参数侧：优先保持 Go 声明类型（聚合参数作为单个 LLVM aggregate 参数），但提供可定位的 frame slot 映射。
2. 返回侧：为支持 `ret_xxx+off(FP)` 写回，把聚合返回拆成可写的结果元素（当前框架要求按结果索引回填）。
3. 先覆盖 `slice`，并纳入 `string/interface`，避免“只对 slice 写死 3 word”的特例风格。

## 当前问题

现有实现在多个位置手写了：

- `string -> (ptr, word)`
- `slice -> (ptr, word, word)`

这类逻辑和 frame slot 绑定在一起，后续如果扩展到更多聚合类型，需要继续复制分支。

## 方案

新增统一的 `framePartsForType(t, goarch)`，由 Go 类型生成 frame 组成部分：

- `string`: 2 段（ptr, word）
- `slice`: 3 段（ptr, word, word）
- `interface`: 2 段（ptr, ptr）

并在 tuple 降级里统一使用：

- `flattenAgg=false`（参数）：
  - LLVM 参数保持 aggregate（例如 `{ ptr, i64, i64 }`）
  - frame slot 记录为 `Index=argIdx, Field=0/1/...`
- `flattenAgg=true`（返回）：
  - LLVM 结果元素按 parts 展开（用于结果槽写回）
  - frame slot 记录为 `Index=argIdx+i, Field=-1`

## 与 cabi 的关系

该改动不直接编码 ABI0/1/2 细节；它只负责把 Go 声明语义映射为稳定的 LLVM 签名与 frame 访问模型。
之后由 cabi 统一做 ABI 模式转换，避免在 plan9 asm 抓签名阶段提前固化调用约定。

## 已做最小实现

- 把 tuple 降级中的 string/slice 特判替换为 `framePartsForType`。
- 增加 `interface` 类型支持：
  - frame parts: `{ptr, ptr}`
  - `llvmTypeForGo(interface)` 返回 `{ ptr, ptr }`
- 增加 `cmd/plan9asmll` 的单元测试：
  - slice 参数（不展开）
  - slice 返回（展开）
  - interface 参数

## 后续

1. 若要覆盖“更多聚合类型”，下一步应把 parts 描述从固定头部扩展为通用类型布局（含 offset/path），并评估 `FrameSlot.Field` 表达能力是否足够。
2. 将 llgo 内部 `internal/build/plan9asm.go` 与本实现保持同构，避免两套签名推导逻辑漂移。
