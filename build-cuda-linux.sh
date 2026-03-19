#!/usr/bin/env bash
set -euo pipefail

CUDA_ROOT="${CUDA_ROOT:-/usr/local/cuda}"
SRC="internal/demod/gpudemod/kernels.cu"
OUT_DIR="internal/demod/gpudemod/build"
OUT_OBJ="$OUT_DIR/kernels.o"

mkdir -p "$OUT_DIR"

if [[ ! -x "$CUDA_ROOT/bin/nvcc" ]]; then
  echo "nvcc not found at $CUDA_ROOT/bin/nvcc" >&2
  exit 1
fi

echo "Building CUDA kernel artifacts for Linux..."
"$CUDA_ROOT/bin/nvcc" -c "$SRC" -o "$OUT_OBJ" -I "$CUDA_ROOT/include"
echo "Built: $OUT_OBJ"
