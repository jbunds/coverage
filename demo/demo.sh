#!/usr/bin/env bash

# this script must be executed within the root of a clone of the github.com/kubernetes/kubernetes repo

# k8s testing-related documentation:
#
#   https://github.com/kubernetes/community/blob/main/contributors/devel/sig-testing/testing.md
#   https://www.kubernetes.dev/docs/guide/contributing/#testing
#   https://github.com/kubernetes/kubernetes/blob/master/hack/make-rules/test.sh

# several tests panic when httptest cannot open sockets due to an insufficient file handle limit,
# so a limit of 2048 is set to accommodate thousands of parallel network sockets:
#
#   https://github.com/kubernetes/kubernetes/blob/03cf1f06d39d261c4194e21e4613a37a0944c895/hack/make-rules/test.sh#L363-L371

ulimit -n 2048

# ensure a clean environment and delegate environment setup to the test runner

unset GOFLAGS
unset CGO_LDFLAGS

# hack/make-rules/test.sh variables

export PARALLEL=4
export KUBE_COVER=y
export KUBE_COVER_REPORT_DIR="${PWD}/coverage"  # `make test` writes here

# local variables
#
# note that GOCOVERDIR must be set:
#
#   https://pkg.go.dev/cmd/covdata
#   https://go.dev/doc/build-cover#running
#   https://go.dev/blog/integration-test-coverage#using-the-integration-test-to-collect-coverage-data

export GOCOVERDIR="${PWD}/covdata"     # `make test` writes binary coverage profile files here
export MERGED="${PWD}/covdata_merged"  # `go tool covdata merge` (below) writes here

rm    -rf "$GOCOVERDIR" "$MERGED" "$KUBE_COVER_REPORT_DIR"
mkdir -p  "$GOCOVERDIR" "$MERGED" "$KUBE_COVER_REPORT_DIR"

# this document explains how to test k8s and collect coverage metrics:
#
#   https://raw.githubusercontent.com/kubernetes/community/refs/heads/main/contributors/devel/sig-testing/testing.md#unit-test-coverage

make test KUBE_COVER=y

# merge the binary coverage data into a unified set

go tool covdata merge -i "$GOCOVERDIR" -o "$MERGED"

# resolve package names to their canonical forms (e.g., k8s.io/kubernetes/pkg/...)
# rather than relying on the file paths observed at test runtime

go tool covdata textfmt -i "$MERGED" -o "${KUBE_COVER_REPORT_DIR}/combined-coverage-normalized.out"
