#!/usr/bin/env bash

bins=("asika" "asikad")
platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "windows/amd64")

for bin in "${bins[@]}"; do
    for platform in "${platforms[@]}"; do
        GOOS=${platform%/*} GOARCH=${platform#*/} \
        go build -ldflags="-s -w" -o "bin/${bin}-${platform//\//-}" ./cmd/${bin} &
    done
done
wait
