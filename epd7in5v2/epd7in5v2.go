// Package epd7in5v2 is an interface for the Waveshare 7.5inch e-paper display V2
// see wiki: https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT
// and code samples:
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/python/lib/waveshare_epd/epd7in5_V2.py
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/c/lib/e-Paper/EPD_7in5_V2.c
// - https://github.com/waveshare/e-Paper/blob/master/RaspberryPi_JetsonNano/c/examples/EPD_7in5_V2_test.c
//
// And device spec: https://www.waveshare.com/w/upload/6/60/7.5inch_e-Paper_V2_Specification.pdf
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
	"log"
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
	IMAGE_PROCESS                  byte = 0x13 // This is "Data Transmission 2" -> "NEW" in KW mode, which which we are
	DUAL_SPI_MODE                  byte = 0x15
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
	SPI_FLASH_CONTROL              byte = 0x65 // This is "Gate/Source Start Setting"
	REVISION                       byte = 0x70
	GET_STATUS                     byte = 0x71
	AUTO_MEASUREMENT_VCOM          byte = 0x80
	READ_VCOM_VALUE                byte = 0x81
	VCM_DC_SETTING                 byte = 0x82
)

// Yanked from the Python example, I don't know what this is yet.
var VOLTAGE_FRAME_7IN5_V2 = [7]byte{
	0x6, 0x3F, 0x3F, 0x11, 0x24, 0x7, 0x17,
}

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

	c, err := port.Connect(4*physic.MegaHertz, spi.Mode0, 8)
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

// Reset / Wake Up
func (e *Epd) Reset() {
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
	e.rst.Out(gpio.Low)
	time.Sleep(200 * time.Millisecond)
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
}

// Send Command Byte
func (e *Epd) sendCommand(cmd byte) {
	e.dc.Out(gpio.Low)
	e.cs.Out(gpio.Low)
	e.c.Tx([]byte{cmd}, nil)
	e.cs.Out(gpio.High)
}

// Send Data Byte, one at a time
func (e *Epd) sendData(data byte) {
	// log.Println("sendData")
	e.dc.Out(gpio.High)
	e.cs.Out(gpio.Low)
	e.c.Tx([]byte{data}, nil)
	e.cs.Out(gpio.High)
}

// Send Data Bytearray, chunked by block.
// (Useful for large payloads. See Python dev examples and spidev.writebytes2())
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

// Pause until display is ready. NB: busy pin is _high_ when idle!
func (e *Epd) waitUntilIdle() {
	for e.busy.Read() == gpio.Low {
		log.Println("Still waiting for idle...")
		time.Sleep(1000 * time.Millisecond)
	}
}

// Init and power on display from sleep.
func (e *Epd) Init() {
	// log.Println("   - Reset")
	e.Reset()
	e.waitUntilIdle()

	// log.Println("   - Send Power Settings")
	e.sendCommand(POWER_SETTING)
	e.sendData(0x17)                     // 1-0=11 internal power
	e.sendData(VOLTAGE_FRAME_7IN5_V2[6]) // VGH&VGL
	e.sendData(VOLTAGE_FRAME_7IN5_V2[1]) // VSH
	e.sendData(VOLTAGE_FRAME_7IN5_V2[2]) // VSL
	e.sendData(VOLTAGE_FRAME_7IN5_V2[3]) // VSHR
	e.waitUntilIdle()

	// log.Println("   - VCM DC")
	e.sendCommand(VCM_DC_SETTING)
	e.sendData(VOLTAGE_FRAME_7IN5_V2[0])
	e.waitUntilIdle()

	// log.Println("   - Booster Soft Start")
	e.sendCommand(BOOSTER_SOFT_START)
	e.sendData(0x27)
	e.sendData(0x27)
	e.sendData(0x2F)
	e.sendData(0x17)
	e.waitUntilIdle()

	// log.Println("   - PLL Control")
	e.sendCommand(PLL_CONTROL)
	// Python example called 0x30 "OSC Setting" but it is the PLL clock freq.
	e.sendData(VOLTAGE_FRAME_7IN5_V2[0]) // 0110 = 50Hz.
	e.waitUntilIdle()

	// log.Println("   - Display Power On")
	e.sendCommand(POWER_ON)
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()

	// log.Println("   - Panel Setting")
	e.sendCommand(PANEL_SETTING)
	e.sendData(0x1F)
	// 0 0 0 1 1 1 1 1
	//     * LUT from OTP so we don't have to send it
	//       * K/W Mode (i.e. black and white, this isn't a red-capable panel)
	//         * * * * Default values
	e.waitUntilIdle()

	// log.Println("   - Resolution Setting")
	e.sendCommand(TCON_RESOLUTION)
	e.sendData(0x03)
	e.sendData(0x20)
	e.sendData(0x01)
	e.sendData(0xE0)
	// Not sure how 800x480 is encoded described in this.
	e.waitUntilIdle()

	// log.Println("   - Set Dual SPI Mode")
	e.sendCommand(DUAL_SPI_MODE)
	e.sendData(0x00)
	// Set as DISABLED
	e.waitUntilIdle()

	// log.Println("   - VCOM and DATA")
	e.sendCommand(VCOM_AND_DATA_INTERVAL_SETTING)
	e.sendData(0x10)
	e.sendData(0x07)
	e.waitUntilIdle()

	// log.Println("   - TCON Setting")
	e.sendCommand(TCON_SETTING)
	e.sendData(0x22)
	e.waitUntilIdle()

	// log.Println("   - Gate/Source Start Setting")
	e.sendCommand(SPI_FLASH_CONTROL) // But Python called 0x65 "Resolution setting"
	// And yes, this is exactly what the Python did, with the comment on the 2nd line
	// I think this is related to rotation...
	e.sendData(0x00)
	e.sendData(0x00) // 800*480
	e.sendData(0x00)
	e.sendData(0x00)
	e.waitUntilIdle()
	// log.Println("   Init Complete")
}

// Clears the screen to white.
// @TODO: Per the docs, 0=black, 1=white, but this works: 0 is white. :confused:
func (e *Epd) Clear() {
	bytes := bytes.Repeat([]byte{0x00}, e.heightByte*e.widthByte)
	e.sendCommand(DATA_START_TRANSMISSION_1)
	e.sendData2(bytes)
	e.sendCommand(DATA_STOP)
	e.sendCommand(IMAGE_PROCESS)
	e.sendData2(bytes)
	e.sendCommand(DATA_STOP)
	e.sendCommand(DISPLAY_REFRESH)
	time.Sleep(5 * time.Second)
	e.waitUntilIdle()
}

// Paint a prepared bitmap in a bytearray to the screen.
func (e *Epd) Display(img []byte) {
	e.sendCommand(IMAGE_PROCESS)
	e.sendData2(img)
	e.sendCommand(DATA_STOP)
	e.sendCommand(DISPLAY_REFRESH)
	time.Sleep(5 * time.Second)
	e.waitUntilIdle()
}

// Sleep the display in power-saving mode.
// Use Init() to wake up and initialize the display.
func (e *Epd) Sleep() {
	e.sendCommand(POWER_OFF)
	e.waitUntilIdle()
	e.sendCommand(DEEP_SLEEP)
	e.sendData(0xA5)
	time.Sleep(2 * time.Second)
}

// Convert the input image into bitmap as a ready-to-display B&W bytearray.
// @TODO: Per the docs, 0=black, 1=white, but this works: 0 is white. :confused:
func (e *Epd) Convert(img image.Image) []byte {
	var byteToSend byte = 0x00
	var bgColor = 1

	buffer := bytes.Repeat([]byte{0x00}, e.widthByte*e.heightByte)

	// Iterate through individual device pixel coords by col within row:
	for j := 0; j < EPD_HEIGHT; j++ {
		for i := 0; i < EPD_WIDTH; i++ {
			bit := bgColor

			// Check that the device pixel we're on is within the image canvas
			if i < img.Bounds().Dx() && j < img.Bounds().Dy() {
				// I flipped this from the original Go pallete. This uses [white=0, black=1]
				// because images were inverted. Something is getting inverted somewhere...
				bit = color.Palette([]color.Color{color.White, color.Black}).Index(img.At(i, j))
			}

			// These two statements do a bitwise shift and OR to pack 8 pixels (as
			// individual bits) into a single byte to send to the display.
			if bit == 1 {
				byteToSend |= 0x80 >> (uint32(i) % 8)
				// Compound operator: `x |= y` is the same as `x = x | y`
				// and the >> is a bitwise right shift
			}

			// If we're on the last pixel in a byte, save the byte and move on
			if i%8 == 7 {
				buffer[(i/8)+(j*e.widthByte)] = byteToSend
				byteToSend = 0x00
			}
		}
	}

	return buffer
}
