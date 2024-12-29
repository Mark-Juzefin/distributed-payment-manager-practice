#!/bin/sh
go mod tidy
go build -C ./src/cmd -o /go/bin/app