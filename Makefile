# libopenapi-validator Makefile
# Targets for formatting, linting, testing, and benchmarking.

.PHONY: all init lint gofumpt import test test-short test-all bench-fast bench-baseline bench-compare

all: gofumpt import lint

init:
	go install mvdan.cc/gofumpt@v0.7.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	go install github.com/daixiang0/gci@v0.13.5
	go install golang.org/x/perf/cmd/benchstat@latest

lint:
	golangci-lint run ./...

gofumpt:
	gofumpt -l -w .

import:
	gci write --skip-generated -s standard -s default -s localmodule -s blank -s dot -s alias .

# Run library tests with race detector (excludes benchmarks/ package)
test:
	go test $$(go list ./... | grep -v /benchmarks) -count=1 -race -timeout=5m

# Run library tests without race detector for faster iteration (excludes benchmarks/ package)
test-short:
	go test $$(go list ./... | grep -v /benchmarks) -count=1 -short -timeout=2m

# Run ALL tests including benchmarks/ package (production spec tests, ~40s extra)
test-all:
	go test ./... -count=1 -race -timeout=5m

# Run the fast benchmark suite (excludes Init and Prod benchmarks)
bench-fast:
	go test -bench='Benchmark(PathMatch|RequestValidation|ResponseValidation|RequestResponseValidation|ConcurrentValidation|Memory)' \
		-benchmem -count=1 -timeout=10m ./benchmarks/

# Run fast suite with count=5 and save as baseline
bench-baseline:
	@mkdir -p benchmarks/results
	go test -bench='Benchmark(PathMatch|RequestValidation|ResponseValidation|RequestResponseValidation|ConcurrentValidation|Memory)' \
		-benchmem -count=5 -timeout=30m ./benchmarks/ \
		| tee benchmarks/results/baseline.txt

# Run fast suite with count=5, save as current, compare against baseline
bench-compare:
	@mkdir -p benchmarks/results
	go test -bench='Benchmark(PathMatch|RequestValidation|ResponseValidation|RequestResponseValidation|ConcurrentValidation|Memory)' \
		-benchmem -count=5 -timeout=30m ./benchmarks/ \
		| tee benchmarks/results/current.txt
	benchstat benchmarks/results/baseline.txt benchmarks/results/current.txt
