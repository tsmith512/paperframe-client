# Paperframe Client

## Install


- Ensure that this script will run as a user called `paperframe` and that it can
  sudo. This can be done by setting the default username in the the RasperryPi
  SD card writer.
- Per the screen manufacturer's instructions, enable the SPI kernel module:
  - Run `sudo raspi-config`
  - `Choose Interfacing Options -> SPI -> Yes Enable SPI interface`
  - _[Their wiki](https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual#Enable_SPI_Interface)_
- Reboot
- Install the latest tarball:
  - @TODO: Update URL once this is merged into trunk.
  - `curl -o - https://raw.githubusercontent.com/tsmith512/paperframe-client/dev/dist/home/paperframe/update.sh | bash`
  - This runs [update.sh](dist/home/paperframe/update.sh), and part of that process
    will be to create a version copy in Paperframe's home directory.

## Credits

This includes a _ton_ of strategy and code from David Eisinger's
[dce/e-paper-frame](https://github.com/dce/e-paper-frame), author of one of the
[tutorials](https://www.viget.com/articles/making-an-email-powered-e-paper-picture-frame/)
I used for the RPi/display portion of the Paperframe project. Many thanks for
sharing and clearly documenting!
