// Docs
// restapi
// processing
// logging
// insrumentation
// configuration

// Testing:
// - Unit tests
// - Integration tests
// *don't forget to use -race when rung tests to detect
// possible reces

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// number of simultanious outgoing HTTP(S) connections
var maxHTTPconnections = 100 // TODO: add cmd-line flag to override
// TODO: add cmd-line flag for 

func main() {
	logInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}
