package www

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type SkipExistingType = int

const (
	AlwaysDownload = iota
	SkipExistingNonZero
	SkipExistingSameSize
)

type ClientOptions struct {
	BaseURL               url.URL
	IndexFetchWorkerCount int
	DownloadWorkerCount   int
	TargetDirectory       string
	SkipExisting          SkipExistingType
}

type Client struct {
	Options ClientOptions

	httpClient *fasthttp.Client
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

	client := Client{
		Options:    options,
		httpClient: httpClient,
	}
	return client
}

func (c *Client) FetchIndexes(target url.URL) ([]string, []*IndexFile) {
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

func (c *Client) fetchIndexesWorker(id int, wg *sync.WaitGroup, targetChan chan url.URL, reportChan chan<- IndexFile) {
	for {
		target, ok := <-targetChan
		if !ok {
			return
		}

		// create target directory
		path := filepath.Join(c.Options.TargetDirectory, target.Path)
		os.MkdirAll(path, 0755)

		dirs, indexFiles := c.FetchIndexes(target)
		for _, dir := range dirs {
			next := url.URL{Scheme: target.Scheme, Host: target.Host, RawQuery: target.RawQuery, Path: dir}
			wg.Add(1)
			targetChan <- next
		}
		for _, indexFile := range indexFiles {
			reportChan <- *indexFile
		}

		wg.Done()
	}
}

func (c *Client) RunFetchIndexesWorker(count int) {
	wg := &sync.WaitGroup{}
	targetChan := make(chan url.URL, 102400)
	reportChan := make(chan IndexFile, 102400)

	for i := 0; i < count; i++ {
		go c.fetchIndexesWorker(i, wg, targetChan, reportChan)
	}

	go func(outDir string, reportChan <-chan IndexFile) {
		logPath := filepath.Join(outDir, ".wwwsync-files.txt")
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		file.Truncate(0)

		writer := bufio.NewWriter(file)
		for {
			select {
			case indexFile, ok := <-reportChan:
				if !ok {
					writer.Flush()
					file.Close()
					return
				}

				writer.WriteString(indexFile.URL.String() + "\n")
				writer.Flush()
				break
			}
		}
	}(c.Options.TargetDirectory, reportChan)

	wg.Add(1)
	targetChan <- c.Options.BaseURL
	wg.Wait()

	for {
		if len(reportChan) == 0 {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	close(targetChan)
}

func (c *Client) download(u *url.URL, targetPath string, bytesTotalChan chan<- int64, bytesRecvChan chan<- int64) error {
	if c.Options.SkipExisting == SkipExistingNonZero {
		if f, err := os.Stat(targetPath); err == nil {
			fSize := f.Size()
			if f.Size() > 0 {
				logger.WithFields(logrus.Fields{
					"size": humanize.Bytes(uint64(fSize)),
				}).Debugf("Skip: %s", u.Path)
				return nil
			}
		}
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	if c.Options.SkipExisting == SkipExistingSameSize {
		if f, err := os.Stat(targetPath); err == nil {
			if size == f.Size() {
				logger.WithFields(logrus.Fields{
					"size": humanize.Bytes(uint64(size)),
				}).Debugf("Skip: %s", u.Path)
				return nil
			}
		}
	}

	bytesTotalChan <- size
	reader := &ProxyReader{Reader: resp.Body}
	reader.SetReadListener(func(diff int64) {
		bytesRecvChan <- diff
	})

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	started := time.Now()
	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	elapsed := time.Since(started)
	logger.WithFields(logrus.Fields{
		"elapsed": elapsed.String(),
		"size":    humanize.Bytes(uint64(size)),
	}).Debugf("Downloaded: %s", u.Path)

	return nil
}

func (c *Client) downloadWorker(wg *sync.WaitGroup, downloadChan chan url.URL, bytesTotalChan chan<- int64, bytesRecvChan chan<- int64, finishChan chan<- url.URL) {
	targetDirectory := c.Options.TargetDirectory

	for {
		u, ok := <-downloadChan
		if !ok {
			return
		}

		targetPath := filepath.Join(targetDirectory, u.Path)
		if err := c.download(&u, targetPath, bytesTotalChan, bytesRecvChan); err != nil {
			logger.Error(err)
		}
		finishChan <- u
		wg.Done()
	}
}

func (c *Client) RunDownloadWorker(count int) {
	logPath := filepath.Join(c.Options.TargetDirectory, ".wwwsync-files.txt")
	file, err := os.OpenFile(logPath, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	wg := &sync.WaitGroup{}
	downloadChan := make(chan url.URL, 10240)
	bytesTotalChan := make(chan int64, 10240)
	bytesRecvChan := make(chan int64, 10240)
	finishChan := make(chan url.URL, 10240)
	for i := 0; i < count; i++ {
		go c.downloadWorker(wg, downloadChan, bytesTotalChan, bytesRecvChan, finishChan)
	}

	var total uint64
	atomic.StoreUint64(&total, 0)

	ctx, cancel := context.WithCancel(context.Background())
	go func(bytesTotalChan <-chan int64, bytesRecvChan <-chan int64, finishChan <-chan url.URL, total *uint64) {
		var bytesTotal int64 = 0
		var bytesReceived int64 = 0
		var bytesReceivedLast int64 = 0
		var finished int64 = 0
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return

			case n := <-bytesTotalChan:
				bytesTotal += n
				break

			case n := <-bytesRecvChan:
				bytesReceived += n
				break

			case <-finishChan:
				finished++
				break

			case <-ticker.C:
				diff := uint64(bytesReceived - bytesReceivedLast)
				bytesReceivedLast = bytesReceived
				logger.WithFields(logrus.Fields{
					"speed": fmt.Sprintf("%s/s", humanize.Bytes(diff)),
				}).Infof("[%d/%d] %s/%s", finished, atomic.LoadUint64(total), humanize.SIWithDigits(float64(bytesReceived), 2, "B"), humanize.SIWithDigits(float64(bytesTotal), 2, "B"))
				break
			}
		}
	}(bytesTotalChan, bytesRecvChan, finishChan, &total)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		u, _ := url.Parse(scanner.Text())
		wg.Add(1)
		atomic.AddUint64(&total, 1)
		downloadChan <- *u
	}

	wg.Wait()
	for {
		if len(downloadChan) == 0 {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	close(downloadChan)
	cancel()

	logger.Info("Done")
}

func (c *Client) Start() error {
	logger.Debug("verbose output is on")
	if err := os.MkdirAll(c.Options.TargetDirectory, 0755); err != nil {
		return err
	}

	c.RunFetchIndexesWorker(c.Options.IndexFetchWorkerCount)
	c.RunDownloadWorker(c.Options.DownloadWorkerCount)
	return nil
}
