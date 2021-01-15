package www

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type ClientOptions struct {
	BaseURL              url.URL
	WorkerCount          int
	TargetDirectory      string
	SkipExistingNonZero  bool
	SkipExistingSameSize bool
}

type Client struct {
	Options ClientOptions

	httpClient *fasthttp.Client

	bytesTotal *int64
	bytesRecv  *int64
}

func NewClient(options ClientOptions) Client {
	if !strings.HasSuffix(options.BaseURL.Path, "/") {
		options.BaseURL.Path += "/"
	}
	options.TargetDirectory, _ = filepath.Abs(options.TargetDirectory)

	httpClient := &fasthttp.Client{
		Name:            "WWWSync",
		MaxConnsPerHost: 0,
	}

	var bytesTotal int64 = 0
	var bytesRecv int64 = 0
	client := Client{
		Options:    options,
		httpClient: httpClient,
		bytesTotal: &bytesTotal,
		bytesRecv:  &bytesRecv,
	}
	return client
}

func (c *Client) FetchIndexes(target url.URL) ([]string, []url.URL) {
	fmt.Println("look:", target.String())
	statusCode, body, err := c.httpClient.Get(nil, target.String())
	if err != nil {
		logger.Panic(err)
	}
	if statusCode != fasthttp.StatusOK {
		logger.Errorf("Unexpected status code: %d")
	}

	hasIndexOfTitle := strings.Contains(string(body), "<title>Index of ")
	logger.WithFields(logrus.Fields{
		"statusCode": statusCode,
		"hasIndexOf": hasIndexOfTitle,
	}).Debugf("Hit: %s", target.Path)
	if !hasIndexOfTitle {
		return nil, nil
	}

	return getURLs(target.String(), body)
}

func (c *Client) fetchIndexesWorker(id int, wg *sync.WaitGroup, targetChan chan url.URL, reportChan chan<- url.URL) {
	for {
		target, ok := <-targetChan
		if !ok {
			return
		}

		dirs, files := c.FetchIndexes(target)
		for _, dir := range dirs {
			next := url.URL{Scheme: target.Scheme, Host: target.Host, RawQuery: target.RawQuery, Path: dir}
			wg.Add(1)
			targetChan <- next
		}
		for _, file := range files {
			reportChan <- file
		}

		wg.Done()
	}
}

func (c *Client) RunFetchIndexesWorker(count int) {
	wg := &sync.WaitGroup{}
	targetChan := make(chan url.URL, 102400)
	reportChan := make(chan url.URL, 10240)

	for i := 0; i < count; i++ {
		go c.fetchIndexesWorker(i, wg, targetChan, reportChan)
	}

	go func(reportChan <-chan url.URL) {
		for {
			select {
			case u := <-reportChan:
				fmt.Println("found:", u.String())
				break
			}
		}
	}(reportChan)

	wg.Add(1)
	targetChan <- c.Options.BaseURL
	wg.Wait()
	close(targetChan)

}

func (c *Client) Start() {
	c.RunFetchIndexesWorker(c.Options.WorkerCount)
}
