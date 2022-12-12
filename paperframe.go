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
	"time"
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
		// Start by determining what to show now
		currentId, err := getCurrentId()
		if err != nil {
			return 1
		}

		image, err := getImage(currentId)
		if err != nil {
			return 1
		}

		displayImage(image, epd)

		log.Println("Waiting for hourly or exit")

		// Channels for system term/int signals and exit code for graceful shutdown
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
		exit := make(chan int, 1)

		// New ticker-on-the-minute and a channel to stop it
		ticker := time.NewTicker(time.Minute)
		stopTicker := make(chan bool, 1)

		// EVERY 10 MIN, CHECK IF ACTIVE IMAGE HAS CHANGED
		go func() {
			for {
				select {
				case currentTime := <-ticker.C:
					if currentTime.Minute()%10 == 0 {
						log.Printf("-> Ten-minute check at %s", currentTime.String())
						checkNewId, err := getCurrentId()

						if err != nil {
							log.Printf("-> Failed to fetch current ID")
						}

						if checkNewId == currentId {
							log.Printf("-> Current image already on display")
						} else {
							currentId = checkNewId

							image, _ := getImage(currentId)
							if image != nil {
								displayImage(image, epd)
							}
						}
					}

				case <-stopTicker:
					ticker.Stop()
					log.Printf("-> Ticker stopped")
				}
			}
		}()

		// CLEAR AND GRACEFUL SHUTDOWN
		go func() {
			received := <-signals
			log.Println(fmt.Sprintf("-> Received signal: %s", received))
			stopTicker <- true
			displayClear(epd)
			exit <- 0
		}()

		return <-exit

	default:
		fmt.Print(README)
		return 1
	}
}

// Fetch the current ID from the API.
func getCurrentId() (string, error) {
	data, err := http.Get(API_ENDPOINT + "/now/id")

	if err != nil || data.StatusCode != 200 {
		log.Println("HTTP Status: " + data.Request.Response.Status)
		return "", errors.New("Unable to fetch current ID. (HTTP " + data.Request.Response.Status + ")")
	}

	id, err := io.ReadAll(data.Body)
	if err != nil {
		return "", errors.New("Unable to decode API response body for current ID.")
	}

	return string(id), nil
}

// Fetch an image to display.
// Backwards compatiblility: if id == "", look up current ID and use that.
func getImage(id string) (image.Image, error) {
	var path string

	if id == "" {
		var err error
		id, err = getCurrentId()
		if err != nil {
			return nil, errors.New("Unable to look up current ID.")
		}
	}

	path = "/image/" + id

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
