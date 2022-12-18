#!/bin/sh

# Download the latest release
wget --content-disposition https://paperframes.net/api/download

# Grab its filename. (This will get the newest file, not the latest number.)
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
