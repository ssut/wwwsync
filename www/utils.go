package www

import (
	"bytes"
	"log"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

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

func getURLs(from string, body []byte) (dirs []string, files []url.URL) {
	u, err := url.Parse(from)
	if err != nil {
		log.Panic(err)
	}

	dirs = []string{}
	files = []url.URL{}

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

			// Check if the token is an <a> tag
			isAnchor := t.Data == "a"
			if !isAnchor {
				continue
			}

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
				file := url.URL{
					Scheme:   u.Scheme,
					Host:     u.Host,
					Path:     path.Join(u.Path, p),
					RawQuery: u.RawQuery,
				}
				files = append(files, file)
			}
		}
	}
}
