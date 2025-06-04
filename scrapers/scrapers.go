package scrapers

import (
	"github.com/playwright-community/playwright-go"
	"net/url"
	"otakucrawler/commons"
	"strings"
)

type Scraper interface {
	GetLinks(page playwright.Page, browser playwright.Browser) []string
	Download(page playwright.Page, browser playwright.Browser, episodeRange string, specificEpisodes string, config commons.DownloadConfig, ffmpegPath string)
}

func GetScraper(link string) Scraper {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(parsedURL.Hostname(), "www.")

	switch {
	case strings.Contains(host, "animesaturn"):
		return &AnimeSaturnScraper{}
	default:
		return nil
	}
}

type AnimeSaturnScraper struct{}

func (s *AnimeSaturnScraper) GetLinks(page playwright.Page, browser playwright.Browser) []string {
	return AnmstrnSearch(page, browser)
}

func (s *AnimeSaturnScraper) Download(page playwright.Page, browser playwright.Browser, episodeRange string, specificEpisodes string, config commons.DownloadConfig, ffmpegPath string) {
	AnmstrnDownload(page, browser, episodeRange, specificEpisodes, config, ffmpegPath)
}
