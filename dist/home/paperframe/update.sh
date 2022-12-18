#!/bin/sh

# @TODO: This will get the newest file by time, not by tarball version.
PACKAGE=$(ls -Art paperframe*.tar.gz | tail -n 1)

sudo tar \
  --no-overwrite-dir --no-same-owner --no-same-permissions \
  -xzf $PACKAGE -C /

sudo systemctl daemon-reload

sudo systemctl restart paperframe

# rm $PACKAGE
