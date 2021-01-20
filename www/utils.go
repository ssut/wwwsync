package www

import (
	"bytes"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"golang.org/x/net/html"
)

type IndexFile struct {
	URL          *url.URL
	LastModified *time.Time
	Size         int64
}

func getHref(t html.Token) (ok bool, href string) {
	// Iterate over all of the Token's attributes until we find an "href"
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	// "bare" return will retrun the variables (ok, href) as defined in
	// the function definition
	return
}

func getURLs(from string, body []byte) (dirs []string, indexFiles []*IndexFile) {
	u, err := url.Parse(from)
	if err != nil {
		log.Panic(err)
	}

	dirs = []string{}
	indexFiles = []*IndexFile{}

	bodyReader := bytes.NewReader(body)
	z := html.NewTokenizer(bodyReader)

	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			// End of the document
			return

		case tt == html.StartTagToken:
			t := z.Token()

			switch t.Data {
			case "a":
				// Extract the href value, if there is one
				ok, p := getHref(t)
				if !ok {
					continue
				}

				// Pass if url is starts with "..", or is an absolute url
				if strings.Index(p, "..") == 0 || strings.Index(p, "://") > -1 {
					continue
				}

				// Directories
				if strings.HasSuffix(p, "/") {
					dir := path.Join(u.Path, p)
					dirs = append(dirs, dir)
				} else {
					u := url.URL{
						Scheme:   u.Scheme,
						Host:     u.Host,
						Path:     path.Join(u.Path, p),
						RawQuery: u.RawQuery,
					}
					indexFile := &IndexFile{
						URL:          &u,
						Size:         -1,
						LastModified: nil,
					}
					indexFiles = append(indexFiles, indexFile)
				}
				break
			}
			break

		case tt == html.TextToken:
			if len(indexFiles) == 0 {
				continue
			}

			text := strings.TrimSpace(string(z.Text()))
			lastIndex := len(indexFiles) - 1
			if indexFiles[lastIndex].LastModified == nil {
				d, err := dateparse.ParseAny(text)
				if err != nil {
					indexFiles[lastIndex].LastModified = &d
				}
			} else if indexFiles[lastIndex].Size == -1 {
			}
			break
		}
	}
}
