#!/usr/bin/env bash

error() {
	echo "Error: $1" >&2
	exit 1
}

SU() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		sudo "$@"
	fi
}

build() {
	env go build -o asikad ./cmd/asikad/main.go
	env go build -o asika ./cmd/asika/main.go
}

clean() {
	rm -rf asika* asikad*
}

distclean() {
	clean
	rm -rf ~/.cache/go-build
	SU rm -rf ~/go
}

build
