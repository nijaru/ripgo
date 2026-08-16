#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
hyperfine_bin=${HYPERFINE:-hyperfine}
fd_bin=${FD_BIN:-fd}
results=${BENCH_RESULTS:-"$repo_root/tmp/bench_find.md"}
fixture=$(mktemp -d "${TMPDIR:-/tmp}/ripgo-find-bench.XXXXXX")
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/ripgo-find-build.XXXXXX")
raw_results=$(mktemp "${TMPDIR:-/tmp}/ripgo-find-results.XXXXXX")
trap 'rm -rf "$fixture" "$build_dir" "$raw_results"' EXIT

command -v "$hyperfine_bin" >/dev/null || {
	echo "hyperfine is required (set HYPERFINE to its path)" >&2
	exit 1
}
command -v "$fd_bin" >/dev/null || {
	echo "fd is required (set FD_BIN to its path)" >&2
	exit 1
}

# Build a deterministic tree with regular files, nested directories, ignored
# subtrees, hidden entries, and links. The fixture is disposable and never
# becomes part of the repository.
for group in $(seq 0 31); do
	group_dir="$fixture/group-$group"
	mkdir -p "$group_dir/ignored" "$group_dir/nested/deep"
	printf 'ignored/\n' >"$group_dir/.gitignore"
	printf 'hidden\n' >"$group_dir/.hidden"
	for file in $(seq 0 255); do
		printf 'package bench\nfunc file%d() {}\n' "$file" >"$group_dir/file-$file.go"
		printf 'text %d\n' "$file" >"$group_dir/file-$file.txt"
		printf 'ignored %d\n' "$file" >"$group_dir/ignored/file-$file.go"
	done
	printf 'package nested\n' >"$group_dir/nested/nested.go"
	printf 'package deep\n' >"$group_dir/nested/deep/deep.go"
done

mkdir -p "$fixture/links"
for link in $(seq 0 7); do
	target_group=$((link + 1))
	ln -s "../group-$target_group" "$fixture/links/group-$link"
done
ln -s ../group-0/file-0.go "$fixture/links/file.go"
ln -s .. "$fixture/group-0/loop"

mkdir -p "$(dirname "$results")"
go build -trimpath -o "$build_dir/ripgo" ./cmd/ripgo

root_q=$(printf '%q' "$fixture")
ripgo_q=$(printf '%q' "$build_dir/ripgo")
fd_q=$(printf '%q' "$fd_bin")

# Keep each pair equivalent: both write NUL-delimited results to /dev/null and
# use the same visible roots, pattern modes, type filters, and depth limits.
benchmark_pair() {
	local name=$1
	local ripgo_command=$2
	local fd_command=$3

	{
		echo "## $name"
		echo
		"$hyperfine_bin" --warmup 2 --runs 8 --export-markdown "$raw_results" "$ripgo_command" "$fd_command"
		cat "$raw_results"
		echo
	} >>"$results"
}

{
	echo "# Finder benchmark"
	echo
	echo "- Date: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
	echo "- Environment: $(go version); $(uname -srm)"
	echo "- fd: $($fd_bin --version)"
	echo "- Fixture: 32 groups × 256 Go/text files, nested directories, ignored and hidden entries, 8 directory links, one file link, and one cycle link"
	echo "- Measurement: hyperfine, 2 warmups and 8 measured runs per pair; warm-cache process execution; NUL output redirected to /dev/null"
	echo
} >"$results"

benchmark_pair "path listing (ignore disabled)" \
	"$ripgo_q find '' --no-ignore --print0 $root_q >/dev/null" \
	"$fd_q --no-ignore --print0 '' $root_q >/dev/null"
benchmark_pair "regex and file type (ignore disabled)" \
	"$ripgo_q find 'file-[0-9]+\\.go$' --no-ignore --type f --print0 $root_q >/dev/null" \
	"$fd_q --no-ignore --type f --print0 'file-[0-9]+\\.go$' $root_q >/dev/null"
benchmark_pair "glob and file type (ignore disabled)" \
	"$ripgo_q find --no-ignore --glob '*.go' --type f --print0 $root_q >/dev/null" \
	"$fd_q --no-ignore --glob --type f --print0 '*.go' $root_q >/dev/null"
benchmark_pair "depth limit and file type (ignore disabled)" \
	"$ripgo_q find '' --no-ignore --type f --max-depth 2 --print0 $root_q >/dev/null" \
	"$fd_q --no-ignore --type f --max-depth 2 --print0 '' $root_q >/dev/null"
benchmark_pair "ignore-heavy tree" \
	"$ripgo_q find '' --print0 $root_q >/dev/null" \
	"$fd_q --no-require-git --print0 '' $root_q >/dev/null"
benchmark_pair "followed symlinks (ignore disabled)" \
	"$ripgo_q find '' --no-ignore --follow-symlinks --print0 $root_q >/dev/null" \
	"$fd_q --no-ignore --follow --print0 '' $root_q >/dev/null"

echo "Results saved to $results"
