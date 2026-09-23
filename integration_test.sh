#!/usr/bin/env bash

# workflow required when modifying non-test *.go files
#
# the changes to the testdata files should be reviewed before committing

go test -race -coverprofile cov.out -skip TestIntegrationTest
go run . -n -gomod go.mod -coverprofile cov.out -path cover
cp cov.out testdata
cp cover/github.com/jbunds/coverage/{assets,flags,interfaces,main,pages,scan,sort,tree,ui}.go.html testdata

go test -race -coverprofile cov.out
go run . -n -gomod go.mod -coverprofile cov.out -path cover
cp cov.out testdata
cp cover/github.com/jbunds/coverage/{assets,flags,interfaces,main,pages,scan,sort,tree,ui}.go.html testdata
