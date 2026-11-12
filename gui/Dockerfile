# syntax=docker/dockerfile:1
# Builds getlatestgo-gui (linux, host arch) without installing the Fyne/cgo
# toolchain on the host. Run from the repo root (gui/go.mod replaces the root
# module with ../):
#
#   docker build -f gui/Dockerfile -o gui .
#
# The binary lands in gui/getlatestgo-gui. Bookworm keeps the glibc floor low
# so the binary runs on older distros too.
FROM golang:1.25-bookworm AS build
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
COPY gui/go.mod gui/go.sum gui/
RUN --mount=type=cache,target=/go/pkg/mod cd gui && go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd gui \
 && go vet ./... \
 && go build -trimpath -ldflags="-s -w" -o /out/getlatestgo-gui .

FROM scratch
COPY --from=build /out/getlatestgo-gui /
