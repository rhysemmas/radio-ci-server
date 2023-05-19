# radio-ci-server

A small go program to flash an arduino running [arduino-lora](https://github.com/rhysemmas/arduino-lora) with the latest version of the code.

## setup

You will need:

* RPI running raspian
* [Go](https://go.dev/doc/install) (version 1.18+)
* [Pio](https://docs.platformio.org/en/latest/core/installation/shell-commands.html#piocore-install-shell-commands)
* [libusb](https://github.com/libusb/libusb/wiki) `apt install libusb-1.0-0`

## install

* Clone the repo
* Run `make` to compile and install the program as a systemd unit
* Update your webhook token in the systemd unit at `/lib/systemd/system/radio-ci-server.service`
* Reload systemd: `sudo systemctl daemon-reload`
* Enable the unit: `sudo systemctl enable radio-ci-server.service`
* Start the unit (or reboot): `sudo systemctl start radio-ci-server.service`
