# syntax=docker/dockerfile:1
# Fluid Debian host probe — see code/actions/templates/Dockerfile.go-workload

FROM golang:1.26-bookworm AS build

ARG VERSION=0.0.0
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
COPY core ./core
COPY cmd ./cmd
COPY internal ./internal
COPY config ./config

RUN go mod download

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X main.Version=${VERSION}" \
        -o /out/workload ./cmd

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/workload /usr/local/bin/fluid-probe
COPY config/schema.yml /etc/fluid-probe/schema.yml

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/fluid-probe"]
CMD ["-config", "/etc/fluid-probe/config.yml"]
