#!/bin/sh
set -e
set -x

/app/gen_code.sh
go mod tidy
go build -C ./src/cmd -o /go/bin/app