#!/bin/sh

VERSION=$(git describe --tags)
PACKAGE=paperframe-$(git branch --show-current)-$VERSION

env GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-X main.version=${VERSION}" -o dist/bin/paperframe

tar -C dist --owner=0 --group=0 -zcf $PACKAGE.tar.gz $(ls -A dist)
