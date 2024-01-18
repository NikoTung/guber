#!/bin/bash

set -e

version=$(git describe --tags --long --always | tr -d '\n') || ""
echo $version
for g in darwin linux windows; do
    suffix=""
    if [ "$GOOS" == "windows" ]; then
        suffix=".exe"
    fi
    GOOS=$g GOARCH=amd64 go build -mod=mod -ldflags "-s -w -X main.version=$version" -o bin/guber-$g-$version$suffix
done

