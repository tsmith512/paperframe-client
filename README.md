# Paperframe Client

## Install

From the manufacturer's instructions, enable the SPI kernel module. ([wiki](https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual#Enable_SPI_Interface))

- `sudo raspi-config`
- `Choose Interfacing Options -> SPI -> Yes Enable SPI interface`
- Reboot
- Install the latest tarball by following the script in
  [update.sh](dist/home/paperframe/update.sh). Part of that download will add a
  copy of that script for future easy use.

## Credits

This includes a _ton_ of strategy and code from David Eisinger's
[dce/e-paper-frame](https://github.com/dce/e-paper-frame), author of one of the
[tutorials](https://www.viget.com/articles/making-an-email-powered-e-paper-picture-frame/)
I used for the RPi/display portion of the Paperframe project. Many thanks for
sharing and clearly documenting!
