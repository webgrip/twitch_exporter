# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24.2
ARG OS=linux
ARG ARCH=amd64

FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./

ARG OS
ARG ARCH
ENV CGO_ENABLED=0
ENV GOOS=${OS}
ENV GOARCH=${ARCH}

RUN go build -buildvcs=false -trimpath -ldflags "-s -w" -o /out/twitch_exporter .

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /out/twitch_exporter /bin/twitch_exporter

EXPOSE 9184
ENTRYPOINT ["/bin/twitch_exporter"]
