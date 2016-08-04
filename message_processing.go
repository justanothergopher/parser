package main

import (
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// parseMentions finds all mentions '@alphanumeric' withing given string
// if none found - returns empty slice
func parseMentions(msg string) []string {
	result := regexp.MustCompile(`@[\p{L}\d_]+`).FindAllString(msg, -1)
	for i := range result {
		result[i] = result[i][1:] // strip leading @ symbol
	}
	if result == nil {
		result = []string{}
	}
	return result
}

// parseEmoticons finds all emoticons '(alhhanumeric)' withing given string
func parseEmoticons(msg string) []string {
	result := regexp.MustCompile(`\([\p{L}\d_]+\)`).FindAllString(msg, -1)
	for i := range result {
		result[i] = result[i][1 : len(result[i])-1] // strip '(' and ')' symbols
	}
	if result == nil {
		result = []string{}
	}
	return result
}

// find all valid urls within given string
// useful link: https://mathiasbynens.be/demo/url-regex
func parseLinks(msg string) []string {
	linkPattern := `(https?|ftp)://(-\.)?([^\s/?\.#-]+\.?)+(/[^\s]*)?`
	return regexp.MustCompile(linkPattern).FindAllString(msg, -1)
}

// recursevely traverse all html nodes starting given one
// and stop when 'title' is found, return its value back
// returns (nil, flase) if not found
func traverse(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild != nil {
			return n.FirstChild.Data, true
		}
	}
	// process all child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result, ok := traverse(c)
		if ok {
			return result, ok
		}
	}

	return "", false
}

// linkProcessingJob incapsulates all data related to link processing.
// Each job contains URL and corresponding timestamps
type linkProcessingJob struct {
	url                 string    // url to be fetched
	queueingTime        time.Time // time the job was put into queue
	startProcessingTime time.Time // time the processing(http.get) started
	endProcessingTime   time.Time // time the processing(http.get) started
}

// linkProcessingResult contains link processing result
type linkProcessingResult struct {
	url                 string    // url
	title               string    // retrieved title
	queueingTime        time.Time // time the job was put into input queue
	startProcessingTime time.Time // time the processing(http.get) started
	endProcessingTime   time.Time // time the processing is over
}

// getHTMLTitle reads content of http respone and attempt
// to find <title>xyz</title> returns false in case of either
// any error or not title
func getHTMLTitle(r io.Reader) (string, bool) {
	doc, err := html.Parse(io.LimitReader(r, 1048576))
	if err != nil {
		return "", false
	}
	return traverse(doc)
}

// fetchURL wraps a call to http.Get with 3 things:
// - 1st: do not exceed max number of simultenious http calls
// - 2nd: track total number of requests as well as in-progress requests
// - 3rd: trak execution start time
func fetchURL(job *linkProcessingJob) (resp *http.Response, err error) {
	global.addURL(job.url)          // ensure # of outgoing http calls does not exceed limits
	defer global.removeURL(job.url) // let others goroutines do their job

	job.startProcessingTime = time.Now()
	return http.Get(job.url)
}

// processFetchingJob accepts job as an input, retreives content of the job.url,
// parses it, creates linkProcessingResult object and puts it into
// outgoing channel.
// can(and should) be invoked in asyn mode
// WaitGroup is used to coordinate completion of multiple routins
func processFetchingJob(job linkProcessingJob, out chan linkProcessingResult, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := fetchURL(&job)

	result := linkProcessingResult{
		url:                 job.url,
		title:               "Failed to get HTML title",
		queueingTime:        job.queueingTime,
		startProcessingTime: job.startProcessingTime,
		endProcessingTime:   job.endProcessingTime,
	}
	// check for any error and return its description (if any)
	if err != nil {
		result.title = err.Error()
	} else {
		defer resp.Body.Close()
		if title, ok := getHTMLTitle(resp.Body); ok {
			result.title = title
		}
	}
	result.endProcessingTime = time.Now()
	out <- result
}

func fetchLinksAsync(in chan linkProcessingJob) chan linkProcessingResult {
	var wg sync.WaitGroup
	out := make(chan linkProcessingResult, len(in))
	for job := range in {
		wg.Add(1)
		go processFetchingJob(job, out, &wg)
	}
	wg.Wait()
	close(out)
	return out
}

// processLinks converts slice of strings into a channel of
// linkProcessingJob and then run those jobs in async mode
// It returns closed channel of linkProcessingResult
func processLinks(links []string) chan linkProcessingResult {
	jobs := make(chan linkProcessingJob, len(links))
	for _, url := range links {
		job := linkProcessingJob{url, time.Now(), time.Time{}, time.Time{}}
		jobs <- job
	}
	close(jobs)
	return fetchLinksAsync(jobs)
}
