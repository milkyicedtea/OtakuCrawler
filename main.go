package main

import (
	"log"
	"net/url"
	"strings"
	"time"
)

var supportedDomains = []string{
	"animesaturn.*",
}

func isSupportedLink(link string) bool {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(parsedURL.Hostname(), "www.")

	for _, domain := range supportedDomains {
		if strings.HasSuffix(domain, ".*") {
			base := strings.TrimSuffix(domain, ".*")
			if strings.HasPrefix(host, base+".") || host == base {
				return true
			}
		} else {
			if host == domain {
				return true
			}
		}
	}
	return false
}

func main() {
	printBanner()

	setupResult := CommonSetup()
	if setupResult.Action == Exit {
		return
	}

	scraper := GetScraper(setupResult.URL)
	if scraper == nil {
		log.Printf("Scraper not available for %s", setupResult.URL)
		return
	}

	linksChannel := make(chan []string)
	switch setupResult.Action {
	case Download:
		scraper.Download(
			setupResult.Page,
			setupResult.Browser,
			setupResult.EpisodeRange,
			setupResult.SpecificEpisodes,
			setupResult.DownloadConfig,
		)
	case Search:
		linksChannel <- scraper.GetLinks(setupResult.Page, setupResult.Browser)
		time.Sleep(1 * time.Second)
		log.Printf("DLC Links: %v", <-linksChannel)
	}

	if setupResult.Browser != nil {
		if err := setupResult.Browser.Close(); err != nil {
			log.Fatalf("could not close browser: %v", err)
		}
	}

	if setupResult.Playwright != nil {
		if err := setupResult.Playwright.Stop(); err != nil {
			log.Fatalf("could not stop Playwright: %v", err)
		}
	}

}
