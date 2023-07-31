package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type SeoData struct {
	URL             string
	Title           string
	H1              string
	MetaDescription string
	StatusCode      int
}
type Parser interface {
	getSEOData(res *http.Response) (SeoData, error)
}
type DefaultParser struct {
}

var userAgents = []string{
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"Googlebot/2.1 (+http://www.googlebot.com/bot.html)",
	"Googlebot/2.1 (+http://www.google.com/bot.html)",
}

func randomUserAgent() string {
	rand.Seed(time.Now().Unix())
	randNum := rand.Int() % len(userAgents)
	return userAgents[randNum]
}

func crawlPage(url string, tokens chan struct{}) (*http.Response, error) {
	tokens <- struct{}{}
	resp, err := makeRequest(url)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func scrapePage(url string, token chan struct{}, parser Parser) (SeoData, error) {
	res, err := crawlPage(url, token)
	if err != nil {
		return SeoData{}, nil
	}
	data, err := parser.getSEOData(res)
	if err != nil {
		return SeoData{}, nil
	}
	return data, nil
}

func (d DefaultParser) getSEOData(resp *http.Response) (SeoData, error) {
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return SeoData{}, nil
	}
	results := SeoData{}
	results.URL = resp.Request.URL.String()
	results.StatusCode = resp.StatusCode
	results.Title = doc.Find("title").Text()
	results.H1 = doc.Find("h1").Text()
	results.MetaDescription = doc.Find("meta[name=description]").First().AttrOr("content", "")
	return results, nil
}
func extractURLs(resp *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, err
	}
	results := []string{}
	sel := doc.Find("loc")
	for i := range sel.Nodes {
		loc := sel.Eq(i)
		result := loc.Text()
		results = append(results, result)
	}
	return results, nil
}

func makeRequest(url string) (*http.Response, error) {

	client := http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", randomUserAgent())
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func isSitemap(urls []string) ([]string, []string) {
	siteMapFiles := []string{}
	pages := []string{}
	for _, page := range urls {
		foundSitemap := strings.Contains(page, "xml")
		if foundSitemap {
			fmt.Print("Found sitemap")
			siteMapFiles = append(siteMapFiles, page)
		} else {
			pages = append(pages, page)
		}
	}
	return siteMapFiles, pages
}
func extractSitemapURLs(startUrl string) []string {
	workList := make(chan []string)
	toCrawl := []string{}
	var n int
	n++
	go func() { workList <- []string{startUrl} }()
	for ; n > 0; n-- {
		list := <-workList
		for _, link := range list {
			go func(link string) {
				res, err := makeRequest(link)
				if err != nil {
					fmt.Print(err)
				}
				urls, err := extractURLs(res)
				if err != nil {
					fmt.Print(err)
				}
				siteMapFiles, pages := isSitemap(urls)
				if siteMapFiles != nil {
					workList <- siteMapFiles
				}
				for _, page := range pages {
					toCrawl = append(toCrawl, page)
				}
			}(link)
		}
	}
	return toCrawl
}

func scrapeURLs(urls []string, parser Parser, cuncurrency int) []SeoData {
	tokens := make(chan struct{}, cuncurrency)
	var n int
	workList := make(chan []string)

	results := []SeoData{}
	go func() { workList <- urls }()

	for ; n > 0; n-- {
		list := <-workList
		for _, url := range list {
			if url != "" {
				n++
				go func() {
					log.Printf("Requesting URLS: %s", url)
					res, err := scrapePage(url, tokens, parser)
					if err != nil {
						fmt.Printf("Error scraping %s: %s", url, err)
					} else {
						results = append(results, res)
					}
					workList <- []string{}
				}()
			}
		}
	}
	return results
}

func ScrapeSitemap(url string, parser Parser, cuncurrency int) []SeoData {
	results := extractSitemapURLs(url)
	res := scrapeURLs(results, parser, cuncurrency)
	return res
}
func main() {
	p := DefaultParser{}
	results := ScrapeSitemap("https://www.quicksprout.com/sitemap.xml", p, 10)
	for _, res := range results {
		fmt.Println(res)
	}
}
