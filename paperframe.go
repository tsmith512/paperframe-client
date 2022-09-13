package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const API_ENDPOINT = "https://paperframe.tsmith.photos/api"

func main() {
	if (len(os.Args) == 1) {
		fmt.Println("What should I do? Options: display")
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "display":
		displayCurrentPhoto()
	}
}

func displayCurrentPhoto() {
	// Get the current photo
	data, err := http.Get(API_ENDPOINT + "/now/image")

	if (err != nil || data.StatusCode != 200) {
		fmt.Printf("%#v\n", err)
		return
	}

	fmt.Printf("%#v\n", data)

	// Convert the body from an ReadCloser to a []byte
	body, err := ioutil.ReadAll(data.Body)

	// The e-ink stuff will take a byte array, as will writing a file, so output
	// the file to the filesystem to test the idea.
	err = os.WriteFile("./temp.jpg", body, 0644)
	fmt.Printf("%#v\n", err)
}
