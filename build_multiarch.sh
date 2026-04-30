#!/usr/bin/env bash

targets=("linux/amd64" "linux/arm64" "darwin/amd64" "windows/amd64")
for t in "${targets[@]}"; do
    IFS='/' read -r goos goarch <<< "$t"
    GOOS=$goos GOARCH=$goarch go build -o bin/asikad-$goos-$goarch ./cmd/asikad &
done
wait
