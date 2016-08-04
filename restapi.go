package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
)

type restHandler struct {
	Path    string           `json:"path"`
	Method  string           `json:"method"`
	Handler http.HandlerFunc `json:"-"`
}

type IM struct {
	Msg string `json:"message"`
}

type URLResponse struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type ServiceResponse struct {
	Mentions  []string      `json:"mentions"`
	Emoticons []string      `json:"emoticons"`
	Links     []URLResponse `json:"links"`
}

var RESTHandlers []restHandler

func init() {
	RESTHandlers = []restHandler{
		restHandler{
			Path: "/", Method: "GET", Handler: defaultHandler,
		},
		restHandler{
			Path: "/api/v1/parse", Method: "POST", Handler: doParsingHandler,
		},
		restHandler{
			Path: "/selftest", Method: "GET", Handler: doSelfTestHandler,
		},
	}

	for _, handler := range RESTHandlers {
		http.HandleFunc(
			handler.Path,
			addLogging(handler.Handler, getFunctionName(handler.Handler)))
	}
}

// getFunctionName returns name of the function passed as a parameter
func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// defaultHandler is used as a 'default' in cases when requested uri
// does not match with any registered handlers
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	// set return type as json to allow automatic processing
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	w.WriteHeader(http.StatusNotImplemented) // 501 RFC2616 https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html

	// return a list of available endpoints
	if err := json.NewEncoder(w).Encode(RESTHandlers); err != nil {
		Error.Println(err)
	}
}

// doParsingHandler accepts json payload (should be compatible with IM type)
func doParsingHandler(w http.ResponseWriter, r *http.Request) {
	var payload IM

	defer r.Body.Close() // free resurces in any case

	// read input payload, set upper limit to 1M to avoid overload
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		Error.Println(err)
		return
	}

	// set return type as json to allow automatic processing
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// try to decode payload
	if err := json.Unmarshal(body, &payload); err != nil {
		// unprocessable entity,
		// http://www.restpatterns.org/HTTP_Status_Codes/422_-_Unprocessable_Entit
		w.WriteHeader(422)
		if err := json.NewEncoder(w).Encode(err); err != nil {
			Error.Println(err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)

	// Call parsing methods
	mentions := parseMentions(payload.Msg)
	emoticons := parseEmoticons(payload.Msg)
	links := parseLinks(payload.Msg)
	// fetch titles
	out := processLinks(links)
	// construct output
	titles := []URLResponse{}
	for r := range out {
		titles = append(titles, URLResponse{r.url, r.title})
	}
	result := ServiceResponse{
		Mentions:  mentions,
		Emoticons: emoticons,
		Links:     titles,
	}

	// return its result to a caller
	if err := json.NewEncoder(w).Encode(result); err != nil {
		Error.Println(err)
	}
}

var selftestURLSet = []string{
	"https://www.bbc.com",
	"http://www.cnn.com",
	"http://www.allboxing.ru",
	"https://www.google.com",
	"https://www.youtube.com",
	"https://www.mail.ru",
	"https://www.msn.com",
	"https://www.facebook.com",
	"https://www.yahoo.com",
	"https://www.amazon.com",
	"https://www.baidu.com",
	"http://www.wikipedia.com",
	"https://www.twitter.com",
	"https://www.live.com",
	"https://www.taobao.com",
	"https://www.linkedin.com",
	"https://www.bing.com",
	"https://www.yandex.ru",
	"https://www.vk.com",
	"https://www.instagram.com",
	"https://www.ebay.com",
	"https://www.pinterest.com",
	"https://www.reddit.com",
	"https://www.netflix.com",
}

/*
	if int(global.getHTTPRequestsTotal()) != numberOfRepeats*len(fetchURLTests) {
		t.Errorf("TestLinkProcessing => %d, expect %d", global.globalCounter, numberOfRepeats*len(fetchURLTests))
	}*/

func doSelfTestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	w.WriteHeader(http.StatusOK)
	out := processLinks(selftestURLSet)

	rr := []ServiceResponse{}
	for result := range out {
		r := ServiceResponse{
			Mentions:  []string{},
			Emoticons: []string{},
			Links:     []URLResponse{},
		}
		r.Links = []URLResponse{
			{
				URL:   result.url,
				Title: result.title,
			},
		}

		rr = append(rr, r)
	}
	b, err := json.Marshal(rr)
	if err == nil {
		w.Write(b)
	} else {
		w.Write([]byte(err.Error()))
	}

}
