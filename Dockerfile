FROM golang:1.23.4-bookworm as dev
WORKDIR /app
COPY go.mod .
COPY go.sum .
COPY install_tools.sh .

RUN /app/install_tools.sh

COPY . .

RUN go env -w CGO_ENABLED=0
RUN /app/build.sh
CMD ["/app/run.sh"]


FROM alpine AS prod
WORKDIR /app
COPY . /source/
COPY --from=dev /go/bin/app /app_bin
CMD ["/app_bin"]