// Package epd7in5v2 is an interface for the Waveshare 7.5inch e-paper display V2
// see wiki: https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT
// and code samples:
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/python/lib/waveshare_epd/epd7in5_V2.py
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/c/lib/e-Paper/EPD_7in5_V2.c
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/c/examples/EPD_7in5_V2_test.c
//
// The GPIO and SPI communication is handled by the awesome Periph.io package;
// no CGO or other dependecy needed.
//
// Go module adapted from https://github.com/dce/rpi/blob/master/epd7in5/epd7in5.go
// original by David Eisinger for his project, which itself is a fork of
// https://github.com/gandaldf/rpi/blob/master/epd7in5/epd7in5.go Many thanks!
//
// His version was for the now discontinued HD display 880x528 with 16-shade
// greyscale, but I don't think I'll have to make too many changes.
// (Famous last words...)

package epd7in5v2

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"time"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

const (
	EPD_WIDTH  int = 800
	EPD_HEIGHT int = 480
)

const (
	PANEL_SETTING                  byte = 0x00
	POWER_SETTING                  byte = 0x01
	POWER_OFF                      byte = 0x02
	POWER_OFF_SEQUENCE_SETTING     byte = 0x03
	POWER_ON                       byte = 0x04
	POWER_ON_MEASURE               byte = 0x05
	BOOSTER_SOFT_START             byte = 0x06
	DEEP_SLEEP                     byte = 0x07
	DATA_START_TRANSMISSION_1      byte = 0x10
	DATA_STOP                      byte = 0x11
	DISPLAY_REFRESH                byte = 0x12
	IMAGE_PROCESS                  byte = 0x13
	LUT_FOR_VCOM                   byte = 0x20
	LUT_BLUE                       byte = 0x21
	LUT_WHITE                      byte = 0x22
	LUT_GRAY_1                     byte = 0x23
	LUT_GRAY_2                     byte = 0x24
	LUT_RED_0                      byte = 0x25
	LUT_RED_1                      byte = 0x26
	LUT_RED_2                      byte = 0x27
	LUT_RED_3                      byte = 0x28
	LUT_XON                        byte = 0x29
	PLL_CONTROL                    byte = 0x30
	TEMPERATURE_SENSOR_COMMAND     byte = 0x40
	TEMPERATURE_CALIBRATION        byte = 0x41
	TEMPERATURE_SENSOR_WRITE       byte = 0x42
	TEMPERATURE_SENSOR_READ        byte = 0x43
	VCOM_AND_DATA_INTERVAL_SETTING byte = 0x50
	LOW_POWER_DETECTION            byte = 0x51
	TCON_SETTING                   byte = 0x60
	TCON_RESOLUTION                byte = 0x61
	SPI_FLASH_CONTROL              byte = 0x65
	REVISION                       byte = 0x70
	GET_STATUS                     byte = 0x71
	AUTO_MEASUREMENT_VCOM          byte = 0x80
	READ_VCOM_VALUE                byte = 0x81
	VCM_DC_SETTING                 byte = 0x82
)

// Epd is a handle to the display controller.
type Epd struct {
	c          conn.Conn
	dc         gpio.PinOut
	cs         gpio.PinOut
	rst        gpio.PinOut
	busy       gpio.PinIO
	widthByte  int
	heightByte int
}

// New returns a Epd object that communicates over SPI to the display controller.
func New(dcPin, csPin, rstPin, busyPin string) (*Epd, error) {
	if _, err := host.Init(); err != nil {
		return nil, err
	}

	// DC pin
	dc := gpioreg.ByName(dcPin)
	if dc == nil {
		return nil, errors.New("spi: failed to find DC pin")
	}

	if dc == gpio.INVALID {
		return nil, errors.New("epd: use nil for dc to use 3-wire mode, do not use gpio.INVALID")
	}

	if err := dc.Out(gpio.Low); err != nil {
		return nil, err
	}

	// CS pin
	cs := gpioreg.ByName(csPin)
	if cs == nil {
		return nil, errors.New("spi: failed to find CS pin")
	}

	if err := cs.Out(gpio.Low); err != nil {
		return nil, err
	}

	// RST pin
	rst := gpioreg.ByName(rstPin)
	if rst == nil {
		return nil, errors.New("spi: failed to find RST pin")
	}

	if err := rst.Out(gpio.Low); err != nil {
		return nil, err
	}

	// BUSY pin
	busy := gpioreg.ByName(busyPin)
	if busy == nil {
		return nil, errors.New("spi: failed to find BUSY pin")
	}

	if err := busy.In(gpio.PullDown, gpio.RisingEdge); err != nil {
		return nil, err
	}

	// SPI
	port, err := spireg.Open("")
	if err != nil {
		return nil, err
	}

	c, err := port.Connect(5*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		port.Close()
		return nil, err
	}

	var widthByte, heightByte int

	if EPD_WIDTH%8 == 0 {
		widthByte = (EPD_WIDTH / 8)
	} else {
		widthByte = (EPD_WIDTH/8 + 1)
	}

	heightByte = EPD_HEIGHT

	e := &Epd{
		c:          c,
		dc:         dc,
		cs:         cs,
		rst:        rst,
		busy:       busy,
		widthByte:  widthByte,
		heightByte: heightByte,
	}

	return e, nil
}

// Reset can be also used to awaken the device.
func (e *Epd) Reset() {
	// log.Println("Reset")
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
	e.rst.Out(gpio.Low)
	time.Sleep(200 * time.Millisecond)
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
}

func (e *Epd) sendCommand(cmd byte) {
	// log.Println("sendCommand")
	e.dc.Out(gpio.Low)
	e.cs.Out(gpio.Low)
	e.c.Tx([]byte{cmd}, nil)
	e.cs.Out(gpio.High)
}

func (e *Epd) sendData(data byte) {
	// log.Println("sendData")
	e.dc.Out(gpio.High)
	e.cs.Out(gpio.Low)
	e.c.Tx([]byte{data}, nil)
	e.cs.Out(gpio.High)
}

// @TODO: This function was in dce's fork but it wasn't in gandalf's original
func (e *Epd) sendData2(data []byte) {
	e.dc.Out(gpio.High)
	e.cs.Out(gpio.Low)

	length := len(data)
	blocksize := 4096

	for start := 0; start < length; start += blocksize {
		end := start + blocksize

		if end > length {
			e.c.Tx(data[start:length], nil)
		} else {
			e.c.Tx(data[start:end], nil)
		}
	}

	e.cs.Out(gpio.High)
}

func (e *Epd) waitUntilIdle() {
	// log.Println("wait until idle")

	for e.busy.Read() == gpio.High {
		// log.Println("waiting...")
		time.Sleep(100 * time.Millisecond)
	}
}

func (e *Epd) turnOnDisplay() {
	e.sendCommand(DISPLAY_REFRESH)
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()
}

// Init initializes the display config.
// It should be only used when you put the device to sleep and need to re-init the device.
func (e *Epd) Init() {
	e.Reset()

	e.waitUntilIdle();
	e.sendCommand(0x12);	// SWRESET
	e.waitUntilIdle();

	e.sendCommand(0x46);	// Auto Write Red RAM
	e.sendData(0xf7);
	e.waitUntilIdle();
	e.sendCommand(0x47);	// Auto Write	B/W RAM
	e.sendData(0xf7);
	e.waitUntilIdle();

	e.sendCommand(0x0C);	// Soft start setting
	e.sendData2([]byte{0xAE, 0xC7, 0xC3, 0xC0, 0x40});

	e.sendCommand(0x01);	// Set MUX as 527
	e.sendData2([]byte{0xAF, 0x02, 0x01});

	e.sendCommand(0x11);	// Data entry mode
	e.sendData(0x01);
	e.sendCommand(0x44);
	e.sendData2([]byte{0x00, 0x00, 0x6F, 0x03});
	e.sendCommand(0x45);
	e.sendData2([]byte{0xAF, 0x02, 0x00, 0x00});

	e.sendCommand(0x3C); // VBD
	e.sendData(0x05); // LUT1, for white

	e.sendCommand(0x18);
	e.sendData(0X80);

	e.sendCommand(0x22);
	e.sendData(0XB1); // Load Temperature and waveform setting.
	e.sendCommand(0x20);
	e.waitUntilIdle();

	e.sendCommand(0x4E); // set RAM x address count to 0;
	e.sendData2([]byte{0x00, 0x00});
	e.sendCommand(0x4F);
	e.sendData2([]byte{0x00, 0x00});
}

// Clear clears the screen.
func (e *Epd) Clear() {
	bytes := bytes.Repeat([]byte{0xff}, e.heightByte * e.widthByte / 8)

	e.sendCommand(0x4F);
	e.sendData2([]byte{0x00, 0x00});
	e.sendCommand(0x24);
	e.sendData2(bytes)
	e.sendCommand(0x26)
	e.sendData2(bytes)
	e.sendCommand(0x22);
	e.sendData(0xF7); // Load LUT from MCU(0x32)
	e.sendCommand(0x20);
	time.Sleep(10);
	e.waitUntilIdle();
}

// Display takes a byte buffer and updates the screen.
func (e *Epd) Display(img []byte) {
	e.sendCommand(0x4F);
	e.sendData2([]byte{0x00, 0x00});
	e.sendCommand(0x24);
	e.sendData2(img)
	e.sendCommand(0x22);
	e.sendData(0xF7); // Load LUT from MCU(0x32)
	e.sendCommand(0x20);
	time.Sleep(10);
	e.waitUntilIdle();
}

// Sleep put the display in power-saving mode.
// You can use Reset() to awaken and Init() to re-initialize the display.
func (e *Epd) Sleep() {
	e.sendCommand(POWER_OFF)
	e.waitUntilIdle()
	e.sendCommand(DEEP_SLEEP)
	e.sendData(0XA5)
}

// Convert converts the input image into a ready-to-display byte buffer.
func (e *Epd) Convert(img image.Image) []byte {
	var byteToSend byte = 0x00
	var bgColor = 1

	buffer := bytes.Repeat([]byte{0x00}, e.widthByte*e.heightByte)

	for j := 0; j < EPD_HEIGHT; j++ {
		for i := 0; i < EPD_WIDTH; i++ {
			bit := bgColor

			// @TODO: I think this is where I need to make changes for a B/W/2-gray...
			if i < img.Bounds().Dx() && j < img.Bounds().Dy() {
				bit = color.Palette([]color.Color{color.Black, color.White}).Index(img.At(i, j))
			}

			if bit == 1 {
				byteToSend |= 0x80 >> (uint32(i) % 8)
			}

			if i%8 == 7 {
				buffer[(i/8)+(j*e.widthByte)] = byteToSend
				byteToSend = 0x00
			}
		}
	}

	return buffer
}
