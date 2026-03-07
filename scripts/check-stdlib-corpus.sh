#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

if ! command -v llc >/dev/null 2>&1; then
  echo "llc not found in PATH" >&2
  exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 not found in PATH" >&2
  exit 1
fi

tmp_root=$(mktemp -d)
trap 'rm -rf "$tmp_root"' EXIT

targets=(
  "linux amd64 x86_64-unknown-linux-gnu"
  "linux arm64 aarch64-unknown-linux-gnu"
  "darwin amd64 x86_64-apple-macosx"
  "darwin arm64 arm64-apple-macosx"
)

for target in "${targets[@]}"; do
  set -- $target
  goos=$1
  goarch=$2
  triple=$3

  echo "==> scan $goos/$goarch"
  json="$tmp_root/$goos-$goarch.json"
  go run ./cmd/plan9asmscan -goos="$goos" -goarch="$goarch" -repo-root . -format json -out "$json"
  python3 - "$json" "$goos/$goarch" <<'PY'
import json
import sys

path, target = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

unsupported = data.get("unsupported", [])
parse_errs = data.get("parse_errs") or []
print(f"scan {target}: packages={data['std_pkgs_with_sfile']} files={data['asm_files']} unsupported={len(unsupported)} parse_errs={len(parse_errs)}")
if unsupported:
    top = ", ".join(f"{item['op']}({item['count']})" for item in unsupported[:12])
    raise SystemExit(f"{target}: unsupported ops remain: {top}")
if parse_errs:
    top = ", ".join(f"{item['File']}: {item['Err']}" for item in parse_errs[:8])
    raise SystemExit(f"{target}: parse errors remain: {top}")
PY

  echo "==> transpile+compile $goos/$goarch"
  out_dir="$tmp_root/$goos-$goarch-ll"
  meta="$tmp_root/$goos-$goarch-meta.json"
  go run -C cmd/plan9asm . transpile -goos="$goos" -goarch="$goarch" -dir "$out_dir" -meta "$meta" std >/dev/null

  ll_count=$(find "$out_dir" -name '*.ll' | wc -l | tr -d ' ')
  if [ "$ll_count" -eq 0 ]; then
    echo "$goos/$goarch: no .ll files generated" >&2
    exit 1
  fi
  echo "compiled corpus $goos/$goarch: ll_files=$ll_count"

  while IFS= read -r ll; do
    obj="${ll%.ll}.o"
    llc -mtriple="$triple" -filetype=obj "$ll" -o "$obj"
  done < <(find "$out_dir" -name '*.ll' | sort)
done
