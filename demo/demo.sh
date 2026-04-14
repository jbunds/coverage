#!/bin/zsh

# this script must be executed within the root of a clone of the github.com/kubernetes/kubernetes repo

ulimit -n 2048

unset CGO_LDFLAGS

export GOFLAGS='-mod=vendor'
export GOCOVERDIR="${PWD}/covdata"
export MERGED="${PWD}/covdata_merged"
export NORMALIZED="${PWD}/combined-coverage-normalized.out"

# https://github.com/kubernetes/kubernetes/blob/master/hack/make-rules/test.sh

export PARALLEL=4
export KUBE_COVER=y
export KUBE_COVER_REPORT_DIR="${PWD}/coverage"
rm -rf $GOCOVERDIR && mkdir -p $GOCOVERDIR

make test KUBE_TEST_ARGS="-v -short -cover -args -test.gocoverdir $GOCOVERDIR"

rm -rf $MERGED && mkdir -p $MERGED

# merge the binary coverage data into a unified set

go tool covdata merge -i $GOCOVERDIR -o $MERGED

# resolve package names to their canonical forms (e.g., k8s.io/kubernetes/pkg/...)
# rather than relying on the file paths observed at test runtime

go tool covdata textfmt -i $MERGED -o $NORMALIZED
