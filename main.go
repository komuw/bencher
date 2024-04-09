package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/conc/pool"
	"golang.org/x/exp/maps"
)

var (
	cli                              = http.Client{Timeout: 5 * time.Second}
	totalBenchmarkRequests           = atomic.Int64{}
	totalBenchmarkRequestSuccess     = atomic.Int64{}
	totalBenchmarkRequestFailure     = atomic.Int64{}
	totalBenchmarkRequestHeaderSize  = atomic.Int64{}
	totalBenchmarkResponseHeaderSize = atomic.Int64{}
	totalBenchmarkResponseBodySize   = atomic.Int64{}
	totalBenchmarkThroughput         = atomic.Int64{}

	errMu     = &sync.Mutex{}
	allErrors error
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(),
			`
bencher benchmarks a url and returns the number of requests sent, together with total bytes sent/received.
The bytes sent/received is inclusive of request line, request headers, response line, response headers & response body.
It only currently works with http GET method.

Example:
bencher -u https://example.com -c 10

Args:
-u string   # URL to send requests to.
-c uint64 # Total number of requests to send.
`,
		)
	}

	var (
		u string
		c uint64
	)
	flag.StringVar(
		&u,
		"u",
		"",
		"URL to send requests to.")
	flag.Uint64Var(
		&c,
		"c",
		0,
		"Total number of requests to send.")
	flag.Parse()

	ur, errP := url.Parse(u)
	if errP != nil {
		panic(errP)
	}
	if ur.Scheme == "" {
		panic("use valid url")
	}
	u = ur.String()

	if c <= 0 {
		panic("use valid count")
	}

	wg := pool.New().WithErrors().WithMaxGoroutines(10) // limit concurrency

	for range c {
		wg.Go(
			func() error {
				reqHeaderSize, resHeaderSize, resBodyLength, throughput, responseCode, err := fetch(u)
				totalBenchmarkRequests.Add(1)
				if err == nil {
					totalBenchmarkRequestHeaderSize.Add(reqHeaderSize)
					totalBenchmarkResponseHeaderSize.Add(resHeaderSize)
					totalBenchmarkResponseBodySize.Add(resBodyLength)
					totalBenchmarkThroughput.Add(throughput)

					if responseCode == 200 {
						totalBenchmarkRequestSuccess.Add(1)
					} else {
						totalBenchmarkRequestFailure.Add(1)

						errMu.Lock()
						defer errMu.Unlock()
						allErrors = errors.Join(allErrors, fmt.Errorf("responseCode: %v", responseCode))
					}
				} else {
					errMu.Lock()
					defer errMu.Unlock()
					allErrors = errors.Join(allErrors, err)
				}

				return nil
			},
		)
	}

	err := wg.Wait()
	if err != nil {
		panic(err)
	}

	fmt.Printf(`
allErrors: %v
totalBenchmarkRequests: %v
totalBenchmarkRequestSuccess: %v
totalBenchmarkRequestFailure: %v
totalBenchmarkRequestHeaderSize: %v bytes.
totalBenchmarkResponseHeaderSize: %v bytes.
totalBenchmarkResponseBodySize: %v bytes.
totalBenchmarkThroughput: %v bytes. <- includes request/response line,headers,body.
`,
		allErrors,
		totalBenchmarkRequests.Load(),
		totalBenchmarkRequestSuccess.Load(),
		totalBenchmarkRequestFailure.Load(),
		totalBenchmarkRequestHeaderSize.Load(),
		totalBenchmarkResponseHeaderSize.Load(),
		totalBenchmarkResponseBodySize.Load(),
		totalBenchmarkThroughput.Load(),
	)
}

func fetch(url string) (reqHeaderSize, resHeaderSize, resBodyLength, throughput int64, responseCode int, _ error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	requestHeaderSize := requestHeaderSize(req)

	res, err := cli.Do(req)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	defer res.Body.Close()

	respBodyLength := res.ContentLength
	b, err := io.ReadAll(res.Body)
	if err == nil {
		if len(b) > int(respBodyLength) {
			respBodyLength = int64(len(b))
		}
	}

	respHeaderSize := responseHeaderSize(res)
	totalResponseSize := respBodyLength + respHeaderSize
	totalThroughput := requestHeaderSize + totalResponseSize

	return requestHeaderSize, respHeaderSize, respBodyLength, totalThroughput, res.StatusCode, nil
}

func requestHeaderSize(req *http.Request) int64 {
	s := ""
	for _, key := range maps.Keys(req.Header) {
		val := req.Header.Get(key)
		s = s + fmt.Sprintf("%v: %v\n", key, val)
	}

	s = fmt.Sprintf("%v %v HTTP/2\n%s", req.Method, req.URL.Path, s) // add request line. eg: `GET /example-api/ HTTP/2`
	// println(s)

	return int64(len(s))
}

func responseHeaderSize(res *http.Response) int64 {
	s := ""
	for _, key := range maps.Keys(res.Header) {
		val := res.Header.Get(key)
		s = s + fmt.Sprintf("%v: %v\n", key, val)
	}

	s = fmt.Sprintf("HTTP/2 %d\n%s", res.StatusCode, s) // add response line. eg; `HTTP/2 200`
	// println(s)

	return int64(len(s))
}
