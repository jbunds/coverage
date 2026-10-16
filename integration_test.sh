#!/usr/bin/env bash

# workflow required when modifying non-test *.go files
#
# the changes to the testdata files should be reviewed before committing

tmpdir=$(mktemp -d) || exit 1
trap 'rm -rf $tmpdir' EXIT

for skip in '-skip TestIntegrationTest' ''; do  #fuglyButFunctional
  go test -race -coverprofile cov.out $skip
  go run . -n -gomod go.mod -coverprofile cov.out -outdir $tmpdir
  cp cov.out testdata
  cp $tmpdir/github.com/jbunds/coverage/*.go.html testdata
done
