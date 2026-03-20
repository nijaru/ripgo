#!/bin/bash
set -e

DATA_DIR="data"
REPO_URL="https://github.com/go-chi/chi/archive/refs/tags/v5.2.1.tar.gz"
REPO_DIR="$DATA_DIR/chi-5.2.1"
BENCH_RESULTS="ai/research/benchmarks.md"

mkdir -p "$DATA_DIR"
mkdir -p "ai/research"

if [ ! -d "$REPO_DIR" ]; then
    echo "Downloading static dataset (chi v5.2.1)..."
    curl -L "$REPO_URL" | tar -xz -C "$DATA_DIR"
fi

echo "Building ripgo..."
go build -o ripgo ./cmd/ripgo

echo "Running benchmarks..."
mkdir -p tmp
HYPERFINE="/opt/homebrew/bin/hyperfine"
$HYPERFINE -N --warmup 3 \
    "./ripgo -q 'func' $REPO_DIR" \
    "rg -q 'func' $REPO_DIR" \
    --export-markdown tmp/bench_run.md

TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
GO_VERSION=$(go version)

{
    echo "## Benchmarks - $TIMESTAMP"
    echo "Environment: $GO_VERSION"
    echo "Dataset: chi v5.2.1"
    cat tmp/bench_run.md
    echo ""
} >> "$BENCH_RESULTS"

echo "Results saved to $BENCH_RESULTS"
