package main

import (
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

type webSite struct {
	domain        string
	pages         pageMap
	urlsInDomain  safeBoolMap
	urlsOutDomain safeBoolMap
}

type webPage struct {
	title         string
	urlsInDomain  []string
	urlsOutDomain []string
}

type safeBoolMap struct {
	urls map[string]bool
	mux  sync.Mutex
}

type pageMap map[string]*webPage

func (page webPage) String() string {
	var s string
	for k, v := range page.urlsInDomain {
		s += fmt.Sprintf("  %v: %v\n", k, v)
	}
	return fmt.Sprintf("%v", s)
}

func (site webSite) String() string {
	var s string
	for k, v := range site.pages {
		s += fmt.Sprintf("%s:\n%+v\n", k, v)
	}
	return fmt.Sprintf("domain: %v\n%v", site.domain, s)
}

func (urlMap safeBoolMap) String() string {
	var s string
	for k, v := range urlMap.urls {
		s += fmt.Sprintf("scraped?:%t: %s\n", v, k)
	}
	return fmt.Sprintf("%v", s)
}

// Crawl uses fetcher to recursively crawl
// pages starting with url, to a maximum of depth.
func Crawl(url string, depth int, site *webSite, wg *sync.WaitGroup) {
	defer wg.Done()
	if depth <= 0 {
		return
	}
	page, err := site.Fetch(url)
	site.urlsInDomain.mux.Lock()
	defer site.urlsInDomain.mux.Unlock()
	if err == nil {
		site.pages[url] = &page
		site.urlsInDomain.urls[url] = true
	} else {
		delete(site.urlsInDomain.urls, url)
		fmt.Println(err)
		return
	}

	fmt.Printf("found: %s %q\n", url, page.title)
	for _, u := range page.urlsInDomain {
		u = strings.TrimSuffix(site.domain, "/") + u
		if _, ok := site.urlsInDomain.urls[u]; !ok {
			site.urlsInDomain.urls[u] = false
			wg.Add(1)
			go Crawl(u, depth-1, site, wg)
		}
	}
	return
}

func (f webSite) Fetch(url string) (webPage, error) {
	// Try to Get a response from url
	var page webPage
	resp, err := http.Get(url)
	if err != nil {
		return page, fmt.Errorf("not found: %s", url)
	}
	// If the page responded pipe the body to a tokenizer and defer closing it
	defer resp.Body.Close()
	z := html.NewTokenizer(resp.Body)

	// validID is a very naive regExp to recognize links in the same domain
	// TODO: make better
	validID := regexp.MustCompile(`^/[^/#][^#]+\z`)

	// Start looping through the tokens
	// this code was inspired by:
	// https://schier.co/blog/2015/04/26/a-simple-web-scraper-in-go.html
	for {
		switch tt := z.Next(); tt {
		case html.StartTagToken:
			switch t := z.Token(); t.Data {
			case "a":
				for _, a := range t.Attr {
					if a.Key == "href" {
						if validID.MatchString(a.Val) {
							page.urlsInDomain = append(page.urlsInDomain, a.Val)
						} else {
							page.urlsOutDomain = append(page.urlsOutDomain, a.Val)
						}
						break
					}
				}
			case "title":
				if len(page.title) == 0 {
					z.Next()
					page.title = z.Token().Data
				}
			}
		case html.ErrorToken:
			// End of the document, we're done
			return page, nil
		}
	}
}

func main() {
	// Create an empty webSite type for crawling
	golangSite := webSite{"https://golang.org/",
		make(pageMap),
		safeBoolMap{urls: make(map[string]bool)},
		safeBoolMap{urls: make(map[string]bool)},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go Crawl(golangSite.domain, 3, &golangSite, &wg)
	wg.Wait()

	fmt.Println(golangSite)
	fmt.Println(golangSite.urlsInDomain)
}
