package main

import (
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"tsmith512/epd7in5v2"
)

const API_ENDPOINT = "https://paperframe.tsmith.photos/api"

const README = `
Usage: paperframe <command>

Supported commands:
  clear        Clear the screen to white
  current      Download the current image and display it
  display [id] Download a specific image ID and display it
  service      Display images, updating hourly, clear on TERM/INT.

`

// Use main() as a wrapper to collect and exit with a status code
func main() {
	defer os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Print(README)
		return 1
	}

	var epd *epd7in5v2.Epd

	if runtime.GOARCH != "arm" {
		log.Println("Skipping screen init: not running on compatible hardware")
	} else {
		// See pinout at https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual#Hardware_connection
		epd, _ = epd7in5v2.New("P1_22", "P1_24", "P1_11", "P1_18")
	}

	switch os.Args[1] {
	case "clear":
		displayClear(epd)
		return 1

	case "current":
		image, err := getImage("")
		if err != nil {
			return 1
		}

		displayImage(image, epd)
		return 0

	case "display":
		if len(os.Args) < 3 {
			log.Println("Missing argument: display requires id")
			return 1
		}

		image, err := getImage(os.Args[2])
		if err != nil {
			return 1
		}

		displayImage(image, epd)
		return 0

	case "service":
		// For now: start by doing what "current" does, then wait for a SIGTERM to
		// clear the screen and exit.
		image, err := getImage("")
		if err != nil {
			return 1
		}

		displayImage(image, epd)

		log.Println("Starting signal listener")

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
		done := make(chan int, 1)

		go func() {
			received := <-signals
			log.Println(fmt.Sprintf("Received signal: %s", received))
			displayClear(epd)
			done <- 0
		}()

		return <-done

	default:
		fmt.Print(README)
		return 1
	}
}

// Fetch an image to display. If id is empty, get the whatever the service
// says is current and use that.
func getImage(id string) (image.Image, error) {
	var path string

	if id == "" {
		path = "/now/image"
	} else {
		path = "/image/" + id
	}

	data, err := http.Get(API_ENDPOINT + path)

	if err != nil || data.StatusCode != 200 {
		log.Println("Unable to fetch image at " + path)
		log.Println("HTTP Status: " + data.Request.Response.Status)
		return nil, errors.New("Unable to fetch image. (HTTP " + data.Request.Response.Status + ")")
	}

	image, err := decodeImage(data.Body, data.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	} else {
		return image, nil
	}
}

// Decode GIF or JPEG image given a mimeType
func decodeImage(data io.Reader, mimeType string) (image.Image, error) {
	switch mimeType {
	case "image/gif":
		image, err := gif.Decode(data)
		if err != nil {
			log.Printf("Error decoding GIF: %s", err)
			return nil, err
		}
		return image, nil

	case "image/jpg", "image/jpeg":
		image, err := jpeg.Decode(data)
		if err != nil {
			log.Printf("Error decoding JPEG: %s", err)
			return nil, err
		}
		return image, nil

	default:
		log.Printf("Image type indeterminate or unsupported")
		return nil, errors.New("Image type indeterminate or unsupported")
	}
}

func displayImage(image image.Image, epd *epd7in5v2.Epd) {
	if epd == nil {
		log.Println("Screen unavailable: skipping display")
		return
	}

	log.Println("-> Reset")
	epd.Reset()

	log.Println("-> Init")
	epd.Init()

	log.Println("-> Displaying")
	epd.Display(epd.Convert(image))

	log.Println("-> Sleep")
	epd.Sleep()
}

func displayClear(epd *epd7in5v2.Epd) {
	if epd == nil {
		log.Println("Screen unavailable: skipping clear")
		return
	}

	log.Println("-> Reset")
	epd.Reset()

	log.Println("-> Init")
	epd.Init()

	log.Println("-> Clear")
	epd.Clear()

	log.Println("-> Sleep")
	epd.Sleep()
}
