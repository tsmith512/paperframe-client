#!/bin/sh

# @TODO: This will get the newest file by time, not by tarball version.
PACKAGE=$(ls -Art paperframe*.tar.gz | tail -n 1)

rm -rf /tmp/paperframe-install
mkdir /tmp/paperframe-install

tar \
  --no-overwrite-dir --no-same-owner --no-same-permissions \
  -C /tmp/paperframe-install -xzf $PACKAGE

rm $PACKAGE

cd /tmp/paperframe-install

# @TODO: This will fail for any  nonexistant parent directory:
sudo find * -type f -exec mv {} /{} \;

sudo systemctl daemon-reload
sudo systemctl enable paperframe
sudo systemctl restart paperframe

cd ~

rm -r /tmp/paperframe-install
