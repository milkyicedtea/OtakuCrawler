package scrapers

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"log"
	"otakucrawler/commons"
	"regexp"
	"strings"
	"sync"
	"time"
)

func AnmstrnSearch(page playwright.Page, browser playwright.Browser) []string {
	episodeButtons, err := page.Locator(".bottone-ep").All()
	if err != nil || len(episodeButtons) == 0 {
		log.Fatalf("could not get entries: %v", err)
	}

	originalPage := page

	var links []string

	for _, entry := range episodeButtons {
		elementHandle, err := entry.ElementHandle()
		if err != nil {
			log.Printf("could not get element handle: %v", err)
			continue
		}

		_, err = elementHandle.Evaluate(`(element) => element.scrollIntoView()`)
		if err != nil {
			log.Printf("could not scroll element: %v", err)
			continue
		}

		// wait for new page to open
		// triggering click on entry
		err = entry.Click(playwright.LocatorClickOptions{Button: playwright.MouseButtonMiddle})
		if err != nil {
			log.Printf("could not middle click: %v", err)
			continue
		}

		// wait for page event to capture new page
		newPageChannel := make(chan playwright.Page)
		go func() {
			// block until we have new page.
			newPageInterface, err := browser.Contexts()[0].WaitForEvent("page")
			if err != nil {
				log.Printf("could not detect new page: %v", err)
				return
			}
			newPageChannel <- newPageInterface.(playwright.Page)
		}()

		// retrieve new page
		newPage := <-newPageChannel
		if newPage == nil {
			log.Fatal("Could not find the new page!")
		}

		// wait for new page to be loaded completely
		err = newPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateDomcontentloaded,
		})
		if err != nil {
			log.Fatalf("could not wait for new page to load: %v", err)
		}

		// find <b> element 'Guarda lo streaming'
		bElementLocator := newPage.Locator("b:text('Guarda lo streaming')")
		if bElementLocator == nil {
			log.Println("Could not find <b> element with the text 'Guarda lo streaming'")
		} else {
			// perform click action on element
			err = bElementLocator.Click(playwright.LocatorClickOptions{Button: playwright.MouseButtonLeft})
			if err != nil {
				log.Printf("could not click <b> element: %v", err)
			}
		}

		// copy current URL from new page
		copiedLink := newPage.URL()
		//fmt.Printf("Copied link: %s\n", copiedLink)
		links = append(links, copiedLink)

		err = newPage.Close()
		if err != nil {
			title, err := newPage.Title()
			if err != nil {
				log.Fatalf("could not get title: %v", err)
			}
			log.Printf("could not close page %v: %v", title, err)
		}

		// switch back to original page
		err = originalPage.BringToFront()
		if err != nil {
			log.Printf("could not switch back to original page: %v", err)
			continue
		}

		time.Sleep(250 * time.Millisecond)
	}
	return links
}

type EpisodeDownload struct {
	Index        int
	VideoUrl     string
	IsHLS        bool
	AnimeName    string
	LanguageType string
}

func extractAnimeName(page playwright.Page) (string, string) {
	selectors := []string{
		".container.anime-title-mobile-as.mb-3.w-100 b",
		".container.anime-title-as.mb-3.w-100 b",
		"div[class*='anime-title'] b", // Fallback - any div containing anime-title
	}

	var animeName, languageType string

	for _, selector := range selectors {
		titleElement := page.Locator(selector).First()
		if titleElement != nil {
			title, err := titleElement.TextContent()
			if err == nil && title != "" {
				title = strings.TrimSpace(title)

				// Extract language type (SUB_ITA or ITA)
				if strings.Contains(strings.ToUpper(title), "SUB ITA") {
					languageType = "SUB_ITA"
					// Remove "Sub ITA" from the title
					animeName = strings.TrimSpace(regexp.MustCompile(`(?i)\s*sub\s*ita\s*$`).ReplaceAllString(title, ""))
				} else if strings.Contains(strings.ToUpper(title), "ITA") {
					languageType = "ITA"
					// Remove "ITA" from the title
					animeName = strings.TrimSpace(regexp.MustCompile(`(?i)\s*ita\s*$`).ReplaceAllString(title, ""))
				} else {
					// Default to SUB_ITA if we can't determine
					languageType = "SUB_ITA"
					animeName = title
				}

				// Clean the anime name for filename use
				animeName = cleanFilename(animeName)

				if animeName != "" {
					fmt.Printf("Extracted anime name: '%s', Language: %s\n", animeName, languageType)
					return animeName, languageType
				}
			}
		}
	}

	// Fallback if we couldn't extract the name
	fmt.Println("Warning: Could not extract anime name from main page, using fallback")
	return "Unknown_Anime", "SUB_ITA"
}

func AnmstrnDownload(page playwright.Page, browser playwright.Browser, episodeRange string, specificEpisodes string, config commons.DownloadConfig, ffmpegPath string) {
	episodeButtons, err := page.Locator(".bottone-ep").All()
	if err != nil || len(episodeButtons) == 0 {
		log.Fatalf("could not get entries: %v", err)
	}

	totalEpisodes := len(episodeButtons)
	fmt.Printf("Total episodes found: %d\n", totalEpisodes)

	// Extract anime name and language type from the main page
	animeName, languageType := extractAnimeName(page)

	// Parse episode range or specific episodes
	var start, end = -1, -1
	var episodeList []int

	if episodeRange != "" {
		var err error
		start, end, err = ParseEpisodeRange(episodeRange)
		if err != nil {
			log.Fatalf("Error parsing episode range: %v", err)
		}

		// Check if the range exceeds available episodes
		if end >= totalEpisodes {
			log.Printf("Warning: Specified range end (%d) exceeds available episodes (%d), will download up to episode %d",
				end+1, totalEpisodes, totalEpisodes)
			end = totalEpisodes - 1
		}
	}

	if specificEpisodes != "" {
		var err error
		episodeList, err = ParseSpecificEpisodes(specificEpisodes)
		if err != nil {
			log.Fatalf("Error parsing specific episodes: %v", err)
		}

		// Check if any specified episodes exceed available episodes
		var validEpisodes []int
		for _, ep := range episodeList {
			if ep >= totalEpisodes {
				log.Printf("Warning: Requested episode %d not available (total: %d)", ep+1, totalEpisodes)
			} else {
				validEpisodes = append(validEpisodes, ep)
			}
		}
		episodeList = validEpisodes

		if len(episodeList) == 0 {
			log.Fatalf("No valid episodes to download after filtering")
		}
	}

	// Create a filtered list of episode indices to process
	var episodesToProcess []int
	for i := 0; i < totalEpisodes; i++ {
		if ShouldProcessEpisode(i, start, end, episodeList) {
			episodesToProcess = append(episodesToProcess, i)
		}
	}

	if len(episodesToProcess) == 0 {
		log.Fatalf("No episodes to process after applying filters")
	}

	fmt.Printf("Will download %d episodes\n", len(episodesToProcess))

	batchSize := config.BatchSize
	var speedPerDownload float64

	if config.MaxSpeedMbps > 0 {
		speedPerDownload = config.MaxSpeedMbps / float64(batchSize)
		fmt.Printf("Using batch size: %d, Speed limit: %.1f Mbps total (%.1f Mbps per download)\n",
			batchSize, config.MaxSpeedMbps, speedPerDownload)
	} else {
		speedPerDownload = 0 // No limit
		fmt.Printf("Using batch size: %d, Speed limit: No limit\n", batchSize)
	}

	// Process episodes in batches
	for batchStart := 0; batchStart < len(episodesToProcess); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(episodesToProcess) {
			batchEnd = len(episodesToProcess)
		}

		currentBatch := episodesToProcess[batchStart:batchEnd]

		fmt.Printf("Processing batch of %d episodes (%d to %d)\n",
			len(currentBatch),
			currentBatch[0]+1,
			currentBatch[len(currentBatch)-1]+1)

		var batchDownloads []EpisodeDownload

		for _, episodeIdx := range currentBatch {
			entry := episodeButtons[episodeIdx]

			elementHandle, err := entry.ElementHandle()
			if err != nil {
				log.Printf("could not get element handle for episode %d: %v", episodeIdx+1, err)
				continue
			}

			_, err = elementHandle.Evaluate(`(element) => element.scrollIntoView()`)
			if err != nil {
				log.Printf("could not scroll element for episode %d: %v", episodeIdx+1, err)
				continue
			}

			err = entry.Click(playwright.LocatorClickOptions{Button: playwright.MouseButtonMiddle})
			if err != nil {
				log.Printf("could not middle click episode %d: %v", episodeIdx+1, err)
				continue
			}

			// Wait for new page to open
			newPageChannel := make(chan playwright.Page)
			go func() {
				newPageInterface, err := browser.Contexts()[0].WaitForEvent("page")
				if err != nil {
					log.Printf("could not detect new page for episode %d: %v", episodeIdx+1, err)
					newPageChannel <- nil
					return
				}
				newPageChannel <- newPageInterface.(playwright.Page)
			}()

			newPage := <-newPageChannel
			if newPage == nil {
				log.Printf("Could not find new page for episode %d", episodeIdx+1)
				continue
			}

			err = newPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateDomcontentloaded,
			})
			if err != nil {
				log.Printf("could not wait for new page to load for episode %d: %v", episodeIdx+1, err)
				continue
			}

			// Find and click streaming button
			bElementLocator := newPage.Locator("b:text('Guarda lo streaming')")
			if bElementLocator == nil {
				log.Printf("Could not find streaming button for episode %d", episodeIdx+1)
			} else {
				err = bElementLocator.Click(playwright.LocatorClickOptions{Button: playwright.MouseButtonLeft})
				if err != nil {
					log.Printf("could not click streaming button for episode %d: %v", episodeIdx+1, err)
				}
			}

			// Wait a bit for the player to load
			time.Sleep(2 * time.Second)

			// Try to extract video URL - first try MP4, then HLS
			var videoUrl string
			var isHLS bool

			// Try MP4 first
			videoSrc, err := newPage.Locator("video source[type='video/mp4']").GetAttribute("src", playwright.LocatorGetAttributeOptions{Timeout: playwright.Float(2000)})
			if err == nil && videoSrc != "" {
				videoUrl = videoSrc
				isHLS = false
				log.Printf("Episode %d found MP4 source: %s", episodeIdx+1, videoUrl)
			} else {
				// Try to extract HLS URL from JavaScript
				hlsUrl, err := extractHLSUrl(newPage)
				if err != nil {
					log.Printf("could not extract video URL for episode %d: %v", episodeIdx+1, err)
					continue
				}
				videoUrl = hlsUrl
				isHLS = true
				log.Printf("Episode %d found HLS source: %s", episodeIdx+1, videoUrl)
			}

			batchDownloads = append(batchDownloads, EpisodeDownload{
				Index:        episodeIdx,
				VideoUrl:     videoUrl,
				IsHLS:        isHLS,
				AnimeName:    animeName,
				LanguageType: languageType,
			})

			// Close the page when done with it
			err = newPage.Close()
			if err != nil {
				log.Printf("could not close page for episode %d: %v", episodeIdx+1, err)
			}

			// Switch back to original page
			err = page.BringToFront()
			if err != nil {
				log.Printf("could not switch back to original page: %v", err)
			}

			time.Sleep(250 * time.Millisecond)
		}

		// Download this batch concurrently
		fmt.Printf("Starting downloads for batch of %d episodes\n", len(batchDownloads))
		var wg sync.WaitGroup

		for _, dl := range batchDownloads {
			wg.Add(1)
			go func(dl EpisodeDownload) {
				defer wg.Done()

				if speedPerDownload > 0 {
					fmt.Printf("Starting download for episode %d (max speed: %.1f Mbps)\n",
						dl.Index+1, speedPerDownload)
				} else {
					fmt.Printf("Starting download for episode %d (no speed limit)\n", dl.Index+1)
				}

				var err error
				if dl.IsHLS {
					err = downloadHLSVideo(dl.VideoUrl, dl.AnimeName, dl.LanguageType, dl.Index+1, ffmpegPath, speedPerDownload)
				} else {
					err = downloadVideo(dl.VideoUrl, speedPerDownload)
				}

				if err != nil {
					log.Printf("Download failed for episode %d: %v", dl.Index+1, err)
				} else {
					fmt.Printf("✅ Completed download for episode %d\n", dl.Index+1)
				}
			}(dl)
		}

		// Wait for all downloads in this batch to complete
		fmt.Println("Waiting for current batch to finish downloading...")
		wg.Wait()
		fmt.Printf("Batch completed!\n")
	}

	fmt.Println("All requested episodes processed and downloaded successfully!")
}
