package main

import (
	"tsmith512/epd7in5v2"

	"fmt"
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
  display  Download the current image and display it
  clear    Wipe the screen

`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(README)
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "display":
		displayCurrentPhoto()
	case "clear":
		displayClear()
	}
}

func displayCurrentPhoto() {
	// Get the current photo
	data, err := http.Get(API_ENDPOINT + "/now/image")

	if err != nil || data.StatusCode != 200 {
		log.Printf("%#v\n", err)
		return
	}

	image, err := jpeg.Decode(data.Body)
	if err != nil {
		log.Printf("Error decoding JPEG: %s", err)
		return
	}

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
