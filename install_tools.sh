#!/bin/sh

go install github.com/cosmtrek/air@v1.49.0 && \
go install github.com/google/wire/cmd/wire@latest && \
go install github.com/pressly/goose/v3/cmd/goose@latest