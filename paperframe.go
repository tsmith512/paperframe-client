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

	"github.com/spf13/viper"
)

var API_ENDPOINT string
var CHECK_FREQ int
var CLEAR_AFTER int
var DEBUG bool
var VERSION string

const README = `
Usage: paperframe <command>

Supported commands:
  clear        Clear the screen to white
  current      Download the current image and display it
  display [id] Download a specific image ID and display it
  service      Display images, updating hourly, clear on TERM/INT.
  version      Print version number and exit.

Further, a configuration file "paperframe.toml" must exist in /etc or
~/. and should declare at least the API endpoint.

`

// Use main() as a wrapper to collect and exit with a status code
func main() {
	defer os.Exit(run())
}

func run() int {
	viper.SetConfigName("paperframe")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc")
	viper.AddConfigPath("$HOME/.paperframe")
	viper.SetDefault("api.endpoint", "https://paperframes.net/api")
	viper.SetDefault("api.frequency", 10)
	viper.SetDefault("debug", false)
	viper.SetDefault("clear_after", 12)
	err := viper.ReadInConfig()

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("Configuration file not found, using defaults.")
		} else {
			log.Printf("Fatal error loading config: %s", err)
			return 1
		}
	}

	API_ENDPOINT = viper.GetString("api.endpoint")
	CHECK_FREQ = viper.GetInt("api.frequency")
	DEBUG = viper.GetBool("debug")
	CLEAR_AFTER = viper.GetInt("clear_after")

	if DEBUG {
		log.Println("Verbose output for debugging")
	}

	if len(os.Args) < 2 {
		fmt.Print(README)
		return 1
	}

	var epd *epd7in5v2.Epd

	if runtime.GOARCH == "arm" {
		// See pinout at https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual#Hardware_connection
		epd, err = epd7in5v2.New("P1_22", "P1_24", "P1_11", "P1_18")

		if err != nil || epd == nil {
			// One of the test devices likes to fail to init the screen and gets stuck
			// perpetually waiting for idle. But restarting the service will fix it...
			log.Printf("Failed to initialize screen: %s", err)
			return 1
		}
	} else {
		log.Println("Skipping screen init: not running on compatible hardware")
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("%s\n", VERSION)
		return 0

	case "clear":
		displayClear(epd)
		return 0

	case "current":
		currentId, err := getCurrentId()
		if err != nil {
			log.Println(err)
			return 1
		}

		image, err := getImage(currentId)
		if err != nil {
			log.Println(err)
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
			log.Println(err)
			return 1
		}

		displayImage(image, epd)
		return 0

	case "service":
		// Systemd has a nasty habit of starting this service after dhcpd has forked
		// but not actually established an address so the initial image check fails.
		// Wait until we have reached the API before moving into the service loop.
		for i := 0; i <= 6; i += 1 {
			if checkConnected() {
				if DEBUG {
					log.Println("Connection to API confirmed")
				}
				break
			}
			time.Sleep(10 * time.Second)
		}

		// Keep track of the last time we refreshed the screen
		lastUpdated := time.Now()

		// Start by determining what to show now
		currentId, err := getCurrentId()
		if err != nil {
			log.Println(err)
		}

		image, err := getImage(currentId)
		if err != nil {
			log.Println(err)
		}

		if image != nil {
			displayImage(image, epd)
		}

		log.Printf("Waiting for next %d-minute check or exit signal.\n", CHECK_FREQ)

		// Channels for system term/int signals and exit code for graceful shutdown
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
		exit := make(chan int, 1)

		// New ticker-on-the-minute and a channel to stop it
		ticker := time.NewTicker(time.Minute)
		stopTicker := make(chan bool, 1)

		// EVERY CHECK_FREQ MIN, CHECK IF ACTIVE IMAGE HAS CHANGED
		go func() {
			for {
				select {
				case currentTime := <-ticker.C:
					if currentTime.Minute()%CHECK_FREQ == 0 {
						if DEBUG {
							log.Printf("-> %d-minute check at %s", CHECK_FREQ, currentTime.String())
						}

						// Check what's on display now:
						checkNewId, err := getCurrentId()

						if err != nil || len(checkNewId) == 0 {
							// HTTP Errors or Network transit errors would both be caught here
							log.Printf("-> Failed to fetch current ID")

							if time.Since(lastUpdated).Hours() >= float64(CLEAR_AFTER) {
								// This likely means the device has gone offline.
								// @TODO: Do we want to show a message or start downloading files?
								fmt.Printf("-> Display unchanged too long. Clearing to prevent burn-in.")
								displayClear(epd)
								lastUpdated = time.Now()
							}

							continue
						}

						if checkNewId == currentId {
							// The image hasn't changed since the last check. This is expected
							// except at the top of the hour or if I manually changed it.
							if DEBUG {
								log.Printf("-> Current image already on display (%s)", currentId)
							}
							if time.Since(lastUpdated).Hours() >= float64(CLEAR_AFTER) {
								// This should not happen unless the Worker cron stopped...
								fmt.Printf("-> Display unchanged too long. Clearing to prevent burn-in.")
								displayClear(epd)
								lastUpdated = time.Now()
							}

							continue
						}

						if DEBUG {
							log.Printf("-> New image ID received: %s", checkNewId)
						}

						image, err := getImage(checkNewId)
						if err != nil {
							log.Printf("-> Image could not be downloaded: %s", err)

							if time.Since(lastUpdated).Hours() >= float64(CLEAR_AFTER) {
								// Somehow we can get the next image ID, but we cannot get the
								// file itself... that is also a case I can't quite figure how
								// we'd get to.
								fmt.Printf("-> Display unchanged too long. Clearing to prevent burn-in.")
								displayClear(epd)
								lastUpdated = time.Now()
							}

							continue
						}

						// New image downloaded; replace and update display
						displayImage(image, epd)
						currentId = checkNewId
						lastUpdated = time.Now()
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

			if DEBUG {
				log.Println(fmt.Sprintf("-> Received signal: %s", received))
			}

			stopTicker <- true
			displayClear(epd)
			lastUpdated = time.Now()
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

	if err != nil {
		// Some kind of networking error (we didn't even get an HTTP response)
		if DEBUG {
			log.Printf("Unable to fetch current image ID: %#v", err)
		}
		return "", errors.New("Unable to fetch current ID. (Networking error)")
	}

	if data.StatusCode != 200 {
		if DEBUG {
			log.Printf("Couldn't fetch current image ID. HTTP %d.", data.StatusCode)
		}
		return "", errors.New(fmt.Sprintf("Unable to fetch current ID. (HTTP %d)", data.StatusCode))
	}

	id, err := io.ReadAll(data.Body)
	if err != nil {
		if DEBUG {
			log.Printf("Couldn't decode response: %s.", string(id))
		}

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

	if err != nil {
		// Some kind of networking error (we didn't even get an HTTP response)
		if DEBUG {
			log.Printf("Unable to fetch image at '%s': %#v", path, err)
		}
		return nil, errors.New("Unable to fetch image. (Networking error)")
	}
	if data.StatusCode != 200 {
		if DEBUG {
			log.Printf("Couldn't fetch image at '%s'. HTTP %d.", path, data.StatusCode)
		}
		return nil, errors.New(fmt.Sprintf("Unable to fetch image. (HTTP %d)", data.StatusCode))
	}

	image, err := decodeImage(data.Body, data.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	} else {
		return image, nil
	}
}

func checkConnected() bool {
	res, err := http.Get(API_ENDPOINT)

	if err != nil {
		if DEBUG {
			log.Printf("Connection check error: %#v", err)
		}
		return false
	}

	if res.StatusCode > 300 {
		if DEBUG {
			log.Printf("Connection check HTTP %d: %#v", res.StatusCode, res)
		}
		return false
	}

	return true
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
		if DEBUG {
			log.Println("Screen unavailable: skipping display")
		}
		return
	}

	if DEBUG {
		log.Println("-> Reset")
	}
	epd.Reset()

	if DEBUG {
		log.Println("-> Init")
	}
	epd.Init()

	if DEBUG {
		log.Println("-> Displaying")
	}
	epd.Display(epd.Convert(image))

	if DEBUG {
		log.Println("-> Sleep")
	}
	epd.Sleep()
}

func displayClear(epd *epd7in5v2.Epd) {
	if epd == nil {
		if DEBUG {
			log.Println("Screen unavailable: skipping clear")
		}
		return
	}

	if DEBUG {
		log.Println("-> Reset")
	}
	epd.Reset()

	if DEBUG {
		log.Println("-> Init")
	}
	epd.Init()

	if DEBUG {
		log.Println("-> Clear")
	}
	epd.Clear()

	if DEBUG {
		log.Println("-> Sleep")
	}
	epd.Sleep()
}
