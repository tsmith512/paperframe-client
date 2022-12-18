#!/bin/sh

PACKAGE=paperframe-$(git branch --show-current)-$(git describe --tags)

env GOOS=linux GOARCH=arm GOARM=5 go build -o dist/bin/paperframe

tar -C dist --owner=0 --group=0 -zcf $PACKAGE.tar.gz $(ls -A dist)
