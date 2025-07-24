FROM golang:1.24.5-bookworm AS dev
WORKDIR /app
COPY go.mod .
COPY go.sum .

COPY . .

RUN go env -w CGO_ENABLED=0
RUN go mod tidy
RUN go build -C ./cmd -o /go/bin/app
CMD ["/go/bin/app"]


FROM alpine AS prod
WORKDIR /app
COPY . /source/
COPY --from=dev /go/bin/app /app_bin
CMD ["/app_bin"]