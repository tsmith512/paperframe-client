package main

import (
	"errors"
	"io"
	"tsmith512/epd7in5v2"

	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"runtime"
)

const API_ENDPOINT = "https://paperframe.tsmith.photos/api"

const README = `
Usage: paperframe <command>

Supported commands:
  clear        Clear the screen to white
  current      Download the current image and display it
  display [id] Download a specific image ID and display it

`

// Use main() as a wrapper to collect and exit with a status code for systemd
func main() {
	exitCode := run()
	log.Printf("Exiting with code %d", exitCode)
	defer os.Exit(exitCode)
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Print(README)
		return 1
	}

	switch os.Args[1] {
	case "clear":
		displayClear()
		return 1

	case "current":
		image, err := getCurrentImage()
		if err != nil {
			return 1
		}

		displayImage(image)
		return 0

	case "display":
		if len(os.Args) < 3 {
			fmt.Print("Missing argument: display requires id")
			return 1
		}
		id := os.Args[2]

		image, err := getImageById(id)
		if err != nil {
			return 1
		}

		displayImage(image)
		return 0

	default:
		fmt.Print(README)
		return 1
	}
}

// Fetch and decode the image that is currently set to active
func getCurrentImage() (image.Image, error) {
	data, err := http.Get(API_ENDPOINT + "/now/image")

	if err != nil || data.StatusCode != 200 {
		log.Println("Unable to fetch current image")
		log.Printf("%#v\n", err)
		return nil, errors.New("Unable to fetch current image")
	}

	image, err := decodeImage(data.Body, data.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	} else {
		return image, nil
	}
}

// Fetch and decode a specific image given its ID
func getImageById(id string) (image.Image, error) {
	data, err := http.Get(API_ENDPOINT + "/image/" + id)

	if err != nil || data.StatusCode != 200 {
		log.Println("Unable to fetch current image")
		log.Printf("%#v\n", err)
		return nil, errors.New("Unable to fetch current image")
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
			log.Printf("Error decoding GIF: %s", err)
			return nil, err
		}
		return image, nil

	default:
		log.Printf("Unable to determine image type")
		return nil, errors.New("Unable to determine the image type")
	}
}

func displayImage(image image.Image) {
	// @TODO: This is a weird place to check for this... move it eventually
	if runtime.GOARCH != "arm" {
		log.Println("Not running on compatible hardware")
		return
	}

	// See pinout at https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual#Hardware_connection
	epd, _ := epd7in5v2.New("P1_22", "P1_24", "P1_11", "P1_18")

	log.Println("-> Reset")
	epd.Reset()

	log.Println("-> Init")
	epd.Init()

	log.Println("-> Displaying")
	epd.Display(epd.Convert(image))

	log.Println("-> Sleep")
	epd.Sleep()
}

func displayClear() {
	// @TODO: Could probably abstract this up to the "router" and pass it in
	// so we only have to define it once.
	epd, _ := epd7in5v2.New("P1_22", "P1_24", "P1_11", "P1_18")

	log.Println("-> Reset")
	epd.Reset()

	log.Println("-> Init")
	epd.Init()

	log.Println("-> Clear")
	epd.Clear()

	log.Println("-> Sleep")
	epd.Sleep()
}

func displaySleep() {
	// @TODO: Could probably abstract this up to the "router" and pass it in
	// so we only have to define it once.
	epd, _ := epd7in5v2.New("P1_22", "P1_24", "P1_11", "P1_18")

	log.Println("-> Reset")
	epd.Reset()

	log.Println("-> Sleep")
	epd.Sleep()
}
