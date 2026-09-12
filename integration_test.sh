#!/usr/bin/env bash

# workflow required when modifying {flags,main,tree}.go
#
# the changes to the testdata files should be reviewed before committing

go test -race -coverprofile cov.out -skip TestIntegrationTest ./...
go run . -gomod go.mod -coverprofile cov.out -path cover
cp cov.out testdata
cp cover/github.com/jbunds/coverage/{flags,main,tree}.go.html testdata

go test -race -coverprofile cov.out ./...
go run . -gomod go.mod -coverprofile cov.out -path cover
cp cov.out testdata
cp cover/github.com/jbunds/coverage/{flags,main,tree}.go.html testdata
