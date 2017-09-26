#!/bin/bash

echo "Please export GOOS and GOARCH for different platforms"
echo "Use linux and amd64 by default"

if [ -z "$GOOS" ]; then
  GOOS=linux
fi

if [ -z "$GOARCH" ]; then
  GOARCH=amd64
fi

mkdir -p $(pwd)/bin

docker run --rm \
  -v $(pwd):/go/src/github.com/huangw5/webwx \
  -v $(pwd)/bin:/go/bin \
  -e GOOS=${GOOS} -e GOARCH=${GOARCH} \
  golang go get -v github.com/huangw5/webwx/...
