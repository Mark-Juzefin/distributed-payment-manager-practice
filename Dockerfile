FROM golang:1.23.4-bookworm as dev
WORKDIR /app
COPY go.mod .
COPY go.sum .

#TODO under script
RUN go install github.com/cosmtrek/air@v1.49.0 && \
    go get github.com/mailru/easyjson && go install github.com/mailru/easyjson/...@latest && \
    go install github.com/google/wire/cmd/wire@latest

COPY . .

RUN /app/gen_code.sh

RUN go env -w CGO_ENABLED=0
RUN /app/build.sh
RUN /app/run.sh


FROM alpine AS prod
WORKDIR /app
COPY . /source/
COPY --from=dev /go/bin/app /app_bin
CMD ["/app_bin"]