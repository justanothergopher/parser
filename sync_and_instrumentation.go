package main

import (
	"encoding/json"
	"expvar"
	"sync"
)

// Global is used to store gloabl-level data and enforce application limits
type Global struct {
	globalCounter   int64             // stores total number of all processed urls
	mutex           *sync.Mutex       // control access to shared resource (fetchInProgress)
	fetchInProgress map[string]string // list of all 'in progress' HTTP requests
	processesLimit  chan string       // used to limit number of concurrent http request]s
	expRequests     *expvar.String    // instrumentation: http requests in progress
	expCounter      *expvar.Int       // instrumentation: # of processed requests (total)
}

var global Global

// addURL adds an URL to 'fetch in progress list' and increase
// a counter of total http requests
// - processLimit is used to limit max number of concurrent
// http requests not to exceed global level (maxHTTPconnections)
// - mutex is used to make modificiation to underliying
// map object as thread safe
func (r *Global) addURL(url string) {
	// ensure we do not exceed limit of http connections
	// by addimg an item to processLimit channel
	// (in case the cahhnel is full, this call will be blocked and
	// put on-hold until any previous request is over and the channel
	// has available slot again)
	r.processesLimit <- url
	// protect all modification by mutex so they are thread-safe
	r.mutex.Lock()
	r.fetchInProgress[url] = "in progress"
	r.globalCounter++
	r.updateExportedVars()
	r.mutex.Unlock()
}

// getHTTPRequestsTotal returns number of total attempted http
// requests (regardles of their status) in thread-safe vay
func (r *Global) getHTTPRequestsTotal() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return int(r.globalCounter)
}

// removeURL removes an URL from 'fetch in progress' list
func (r *Global) removeURL(url string) {
	// protect all modification by mutex so they are thread-safe
	r.mutex.Lock()
	delete(r.fetchInProgress, url)
	r.updateExportedVars()
	r.mutex.Unlock()

	// once an URL's fetch is completed - free channel to
	// allow others go-routings to proceed
	<-r.processesLimit
}

// updateExportedVars updates all exported vars using internal
// variables/counters as a source (not theread-safe, thus should be
// called from thread-safe environment)
func (r *Global) updateExportedVars() {
	r.expCounter.Set(r.globalCounter)
	j, _ := json.Marshal(r.fetchInProgress)
	r.expRequests.Set(string(j))
}
