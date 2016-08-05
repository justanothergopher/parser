// Parser contains the following modules
// REST API: restapi.go
// Message parsing: message_processing.go
// Loggin: logger.go
// Synchronization and Insrumentation: sync_and_instrumentation.go
//   /debug/vars - for runtime status
// Testing:
// - Unit tests: parser_test.go
// - Integration tests:
//   /bulktest
//   /selftest
// *don't forget to use -race when rung tests to detect possible reces

package main

import (
	"expvar"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

// address and port to listen to
var serviceAddr = "127.0.0.1:8000"

// number of simultanious outgoing HTTP(S) connections
var maxHTTPconnections = 100

func init() {
	// process cmd-line flags (if any)
	serviceAddr = *flag.String("addr", serviceAddr, "specify addr:port the server should listen on")
	maxHTTPconnections = *flag.Int("max-http-req", maxHTTPconnections, "specify max number of outgoing concurrent http requests")
	flag.Parse()

	// initialize global, will be used by all others routines in run-time
	global = Global{
		globalCounter:   0,
		mutex:           &sync.Mutex{},
		fetchInProgress: make(map[string]string, maxHTTPconnections),
		processesLimit:  make(chan string, maxHTTPconnections),
		expRequests:     expvar.NewString("requests"),
		expCounter:      expvar.NewInt("counter"),
	}
}

func main() {
	logInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
	log.Fatal(http.ListenAndServe(serviceAddr, nil))
}
