package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type TestMatrix struct {
	in  string
	out []string
}

var mentionTests = []TestMatrix{
	{"@test", []string{"test"}},
	{"@TesT", []string{"TesT"}},
	{"@test @test1", []string{"test", "test1"}},
	{"@@@test", []string{"test"}},
	{"@test@test1@test2", []string{"test", "test1", "test2"}},
	{"test @mention@", []string{"mention"}},
	{"@тест", []string{"тест"}},
	{"test", []string{}},
	{"@@@", []string{}},
}

var emoticonTests = []TestMatrix{
	{"(happy)", []string{"happy"}},
	{"(test} (test1)", []string{"test1"}},
	{"(test) (test1)", []string{"test", "test1"}},
	{"(test} (test1", []string{}},
	{"(te(st) (test1", []string{"st"}},
	{"(test} ()test(t)", []string{"t"}},
}

var linkTests = []TestMatrix{
	{"https://foo.com/blah_blah", []string{"https://foo.com/blah_blah"}},
	{"http://foo.com/blah_blah/", []string{"http://foo.com/blah_blah/"}},
	{"http://142.42.1.1:8080/", []string{"http://142.42.1.1:8080/"}},
	{"http://../", []string{}},
}

type ParsingFunc func(string) []string

func testStringProcessingFunc(f ParsingFunc, tests []TestMatrix, t *testing.T) {
	for _, test := range tests {
		match := true
		result := f(test.in)
		if len(test.out) != len(result) {
			match = false
		} else {
			for i := range test.out {
				if test.out[i] != result[i] {
					match = false
					break
				}
			}
		}
		if !match {
			t.Errorf("%q(%q) => %q, expect %q", getFunctionName(f), test.in, result, test.out)
		}
	}
}

func TestMentionsParsing(t *testing.T) {
	testStringProcessingFunc(parseMentions, mentionTests, t)
}

func TestEmoticonsParsing(t *testing.T) {
	testStringProcessingFunc(parseEmoticons, emoticonTests, t)
}

func TestLinksParsing(t *testing.T) {
	testStringProcessingFunc(parseLinks, linkTests, t)
}

var findTitleTests = []struct {
	in  string
	out string
}{
	{"<title My title</title>", ""},
	{"<title>My title</title>", "My title"},
	{"<title1>My title</title>", ""},
	{"<title>My title</title1>", "My title</title1>"},
	{"<title>My title<title>", "My title<title>"},
	{"<title><title>My title</title>", "<title>My title"},
}

func TestGetHTMLTitle(t *testing.T) {
	// positive flow
	for _, test := range findTitleTests {
		title, _ := getHTMLTitle(strings.NewReader(test.in))
		if title != test.out {
			t.Errorf("%q(%q) => %q, expect %q", getFunctionName(getHTMLTitle), test.in, title, test.out)
		}
	}
}

func TestFetchURL(t *testing.T) {
	testMsg := "<html><title>My title</title></html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, testMsg)
	}))
	defer ts.Close()

	job := &linkProcessingJob{
		url:                 ts.URL,
		queueingTime:        time.Now(),
		startProcessingTime: time.Time{},
		endProcessingTime:   time.Time{},
	}

	resp, err := fetchURL(job)
	if err != nil {
		t.Errorf("Error in %q(): %q\n", getFunctionName(fetchURL), err.Error())
	} else {
		b, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		actualMsg := strings.TrimSpace(string(b))
		if testMsg != actualMsg {
			t.Errorf("Error in %q() => %q, expect %q\n", getFunctionName(fetchURL), actualMsg, testMsg)
		}
	}
}

func TestProcessFetchingJob(t *testing.T) {
	testMsg := "<html><title>My title</title></html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, testMsg)
	}))
	defer ts.Close()

	job := linkProcessingJob{
		url:                 ts.URL,
		queueingTime:        time.Now(),
		startProcessingTime: time.Time{},
		endProcessingTime:   time.Time{},
	}

	repeats := 10
	ch := make(chan linkProcessingResult, repeats)
	var wg sync.WaitGroup
	global.globalCounter = 0

	for i := 0; i < repeats; i++ {
		wg.Add(1)
		go processFetchingJob(job, ch, &wg)
	}
	wg.Wait()
	close(ch)

	if len(ch) != repeats {
		t.Errorf("Error in %q() => %q results expect %q\n", getFunctionName(processFetchingJob), len(ch), repeats)
	}

	// negative path
	wg.Add(1)
	job.url = "https://www.wrongurl12345678.com.xyz"
	ch = make(chan linkProcessingResult, 1)
	go processFetchingJob(job, ch, &wg)
	wg.Wait()
	close(ch)
}

func TestFetchLinksAsync(t *testing.T) {
	testMsg := "<html><title>My title</title></html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, testMsg)
	}))
	defer ts.Close()
	in := make(chan linkProcessingJob, 1)
	job := linkProcessingJob{
		url:                 ts.URL,
		queueingTime:        time.Now(),
		startProcessingTime: time.Time{},
		endProcessingTime:   time.Time{},
	}
	in <- job
	close(in)
	out := fetchLinksAsync(in)
	if res := <-out; res.title != "My title" {
		t.Errorf("Error in %q() => %q expect %q\n", getFunctionName(processFetchingJob), res.title, "My title")
	}
}

func TestProcessLinks(t *testing.T) {
	testMsg := "<html><title>My title</title></html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, testMsg)
	}))
	defer ts.Close()
	out := processLinks([]string{ts.URL})
	if len(out) > 1 {
		t.Errorf("Error in %q(%s) => %d result expect %q\n", getFunctionName(processLinks), ts.URL, len(out), 1)
	}
	for result := range out {
		/*d := result.endProcessingTime.Sub(result.startProcessingTime)
		if d > time.Millisecond {
			t.Errorf("Error in %q(%s) => took too long: %dms expect: %dms\n", getFunctionName(processLinks), ts.URL, d/time.Millisecond, time.Millisecond)
		}*/
		if result.title != "My title" {
			t.Errorf("Error in %q(%s) => %q expect %q\n", getFunctionName(processLinks), ts.URL, result.title, "My title")
		}
	}

}
