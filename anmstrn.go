package main

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	Index    int
	VideoUrl string
}

func AnmstrnDownload(page playwright.Page, browser playwright.Browser, episodeRange string, specificEpisodes string) {
	episodeButtons, err := page.Locator(".bottone-ep").All()
	if err != nil || len(episodeButtons) == 0 {
		log.Fatalf("could not get entries: %v", err)
	}

	totalEpisodes := len(episodeButtons)
	fmt.Printf("Total episodes found: %d\n", totalEpisodes)

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

	const batchSize = 3

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

			// Extract video URL
			videoSrc, err := newPage.Locator("video source[type='video/mp4']").GetAttribute("src")
			if err != nil {
				log.Printf("could not get video source for episode %d: %v", episodeIdx+1, err)
				continue
			}

			log.Printf("Episode %d video URL: %s", episodeIdx+1, videoSrc)

			batchDownloads = append(batchDownloads, EpisodeDownload{Index: episodeIdx, VideoUrl: videoSrc})

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

				fmt.Printf("Starting download for episode %d\n", dl.Index+1)
				err := downloadVideo(dl.VideoUrl)
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

func downloadVideo(videoURL string) error {
	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	pathSegments := strings.Split(parsedURL.Path, "/")
	if len(pathSegments) < 2 {
		return fmt.Errorf("URL path too short to determine folder/filename")
	}

	filename := pathSegments[len(pathSegments)-1]
	subfolder := pathSegments[len(pathSegments)-2]

	outputDir := filepath.Join("OtakuCrawler Downloads", subfolder)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, filename)

	// Check if file already exists
	if fileInfo, err := os.Stat(outputPath); err == nil {
		// File exists, check its size
		existingSize := fileInfo.Size()

		// Make a HEAD request to get the expected file size
		resp, err := http.Head(videoURL)
		if err != nil {
			// If we can't determine the size, just download again to be safe
			fmt.Printf("⚠️ Could not check file size for %s, downloading again\n", filename)
		} else {
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					_ = fmt.Errorf("could not close response body: %w", err)
				}
			}(resp.Body)

			expectedSize := resp.ContentLength
			if expectedSize > 0 && existingSize >= expectedSize {
				// File is complete, no need to download again
				fmt.Printf("✅ File already exists with correct size: %s (%.2f MB)\n",
					outputPath, float64(existingSize)/(1024*1024))
				return nil
			}
		}

		// File exists but is incomplete/different, will be overwritten
		fmt.Printf("⚠️ File exists but appears incomplete: %s, downloading again\n", filename)
	}

	// Start downloading the file
	fmt.Printf("⏬ Downloading %s...\n", filename)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer func(outFile *os.File) {
		err := outFile.Close()
		if err != nil {
			_ = fmt.Errorf("could not close output file: %w", err)
		}
	}(outFile)

	resp, err := http.Get(videoURL)
	if err != nil {
		return fmt.Errorf("HTTP error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			_ = fmt.Errorf("could not close response body: %w", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Add a simple progress indicator
	contentLength := resp.ContentLength
	if contentLength > 0 {
		fmt.Printf("Total size: %.2f MB\n", float64(contentLength)/(1024*1024))
	}

	startTime := time.Now()
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("could not write to file: %w", err)
	}

	elapsed := time.Since(startTime).Seconds()
	speed := float64(written) / elapsed / 1024 / 1024 // MB/s

	fmt.Printf("✅ Downloaded to: %s (%.2f MB at %.2f MB/s)\n",
		outputPath, float64(written)/(1024*1024), speed)
	return nil
}
