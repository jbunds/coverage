#!/usr/bin/env bash

# workflow required when modifying non-test *.go files
#
# the changes to the testdata files should be reviewed before committing

for skip in ' -skip TestIntegrationTest' ''; do
  go test -race -coverprofile cov.out $skip
  go run . -n -gomod go.mod -coverprofile cov.out -outdir cover
  cp cov.out testdata
  cp cover/github.com/jbunds/coverage/*.go.html testdata
done
