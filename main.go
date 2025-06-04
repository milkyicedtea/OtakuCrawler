package main

import (
	"log"
	"otakucrawler/commons"
	"otakucrawler/scrapers"
	"time"
)

func main() {
	printBanner()

	setupResult := commons.CommonSetup()
	if setupResult.Action == commons.Exit {
		return
	}

	scraper := scrapers.GetScraper(setupResult.URL)
	if scraper == nil {
		log.Printf("Scraper not available for %s", setupResult.URL)
		return
	}

	linksChannel := make(chan []string)
	switch setupResult.Action {
	case commons.Download:
		scraper.Download(
			setupResult.Page,
			setupResult.Browser,
			setupResult.EpisodeRange,
			setupResult.SpecificEpisodes,
			setupResult.DownloadConfig,
			setupResult.FFmpegPath,
		)
	case commons.Search:
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
