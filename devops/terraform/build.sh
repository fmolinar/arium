#!/bin/sh
# Builds the collector for the Lambda provided.al2023 arm64 runtime into
# build/bootstrap, which main.tf zips. Uses the local Go toolchain, or the
# golang image when Go isn't installed.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
backend=$(cd "$here/../../backend" && pwd)
mkdir -p "$here/build"

# lambda.norpc drops aws-lambda-go's legacy RPC mode, unused by custom runtimes.
build='CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/collector'

if command -v go >/dev/null 2>&1; then
  cd "$backend"
  eval "$(echo "$build" | sed "s#/out/#$here/build/#")"
else
  # Reuse the host's module cache when there is one.
  modcache="${GOMODCACHE:-$HOME/go/pkg/mod}"
  mkdir -p "$modcache"
  docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOPATH=/tmp/go -e GOCACHE=/tmp/gocache \
    -v "$modcache:/tmp/go/pkg/mod" -v "$backend:/src" -v "$here/build:/out" -w /src golang:1.25 sh -c "$build"
fi

echo "built $here/build/bootstrap"
