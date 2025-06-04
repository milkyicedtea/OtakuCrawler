package scrapers

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func cleanFilename(filename string) string {
	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	cleaned := re.ReplaceAllString(filename, "_")

	// Remove multiple consecutive underscores
	re2 := regexp.MustCompile(`_+`)
	cleaned = re2.ReplaceAllString(cleaned, "_")

	// Trim underscores from start and end
	cleaned = strings.Trim(cleaned, "_")

	// Limit length
	if len(cleaned) > 100 {
		cleaned = cleaned[:100]
	}

	return cleaned
}

func extractHLSUrl(page playwright.Page) (string, error) {
	// Get the page content
	content, err := page.Content()
	if err != nil {
		return "", fmt.Errorf("could not get page content: %w", err)
	}

	// Look for jwplayer setup with file parameter
	re := regexp.MustCompile(`file:\s*["']([^"']*\.m3u8[^"']*)["']`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Alternative pattern for different setups
	re2 := regexp.MustCompile(`["']([^"']*\.m3u8[^"']*)["']`)
	matches2 := re2.FindAllStringSubmatch(content, -1)
	for _, match := range matches2 {
		if len(match) >= 2 && strings.Contains(match[1], ".m3u8") {
			return match[1], nil
		}
	}

	return "", fmt.Errorf("could not find HLS URL in page content")
}

func downloadVideo(videoURL string, maxSpeedMbps float64) error {
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
	if maxSpeedMbps > 0 {
		fmt.Printf("⏬ Downloading %s (max speed: %.1f Mbps)...\n", filename, maxSpeedMbps)
	} else {
		fmt.Printf("⏬ Downloading %s (no speed limit)...\n", filename)
	}

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
	var written int64

	// Choose between rate-limited and unlimited download
	if maxSpeedMbps > 0 {
		// Rate-limited download
		maxBytesPerSecond := int(maxSpeedMbps * 1024 * 1024 / 8)         // mbps -> bytes/sec
		fmt.Printf("Rate limiting to %d bytes/sec\n", maxBytesPerSecond) // Debug output

		rateLimitedReader := NewTokenBucketRateLimitedReader(resp.Body, maxBytesPerSecond)
		defer func(rateLimitedReader *TokenBucketRateLimitedReader) {
			err := rateLimitedReader.Close()
			if err != nil {
				_ = fmt.Errorf("could not close limited reader: %w", err)
			}
		}(rateLimitedReader)

		written, err = io.Copy(outFile, rateLimitedReader)
	} else {
		// Unlimited download - direct copy
		written, err = io.Copy(outFile, resp.Body)
	}

	if err != nil {
		return fmt.Errorf("could not write to file: %w", err)
	}

	elapsed := time.Since(startTime).Seconds()
	speed := float64(written) / elapsed / 1024 / 1024 // MB/s

	fmt.Printf("✅ Downloaded to: %s (%.2f MB at %.2f MB/s)\n",
		outputPath, float64(written)/(1024*1024), speed)
	return nil
}

func downloadHLSVideo(hlsUrl, animeName, languageType string, episodeNum int, ffmpegPath string, maxSpeedMbps float64) error {
	// Check if ffmpeg is available
	var ffmpegCmd string
	if ffmpegPath != "" {
		ffmpegCmd = ffmpegPath
	} else {
		// Fallback to system ffmpeg
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			return fmt.Errorf("ffmpeg not found. Please install ffmpeg or ensure it's in your PATH")
		}
		ffmpegCmd = "ffmpeg"
	}

	// Create output directory
	outputDir := filepath.Join("OtakuCrawler Downloads", animeName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	// Generate filename in the requested format: AnimeName_Ep_XX_SUB_ITA/ITA
	filename := fmt.Sprintf("%s_Ep_%02d_%s.mp4", strings.Join(strings.Fields(animeName), ""), episodeNum, languageType)
	outputPath := filepath.Join(outputDir, filename)

	// Check if file already exists and is complete
	if fileInfo, err := os.Stat(outputPath); err == nil && fileInfo.Size() > 1024*1024*10 { // At least 10MB
		fmt.Printf("✅ File already exists: %s (%.2f MB)\n",
			outputPath, float64(fileInfo.Size())/(1024*1024))
		return nil
	}

	// Display download info with speed limit
	if maxSpeedMbps > 0 {
		fmt.Printf("⏬ Downloading HLS stream %s (max speed: %.1f Mbps)...\n", filename, maxSpeedMbps)
	} else {
		fmt.Printf("⏬ Downloading HLS stream %s (no speed limit)...\n", filename)
	}

	startTime := time.Now()

	// Use custom rate-limited HLS downloader instead of direct ffmpeg
	err := downloadHLSWithCustomRateLimit(hlsUrl, outputPath, ffmpegCmd, maxSpeedMbps)
	if err != nil {
		return fmt.Errorf("HLS download failed: %w", err)
	}

	// Check if file was created successfully
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("output file not found after download: %w", err)
	}

	elapsed := time.Since(startTime).Seconds()
	sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	actualSpeedMbps := (sizeMB * 8) / elapsed // Calculate actual speed in Mbps

	fmt.Printf("✅ Downloaded to: %s (%.2f MB in %.1f seconds, %.1f Mbps)\n",
		outputPath, sizeMB, elapsed, actualSpeedMbps)
	return nil
}

func downloadHLSWithCustomRateLimit(hlsUrl, outputPath, ffmpegCmd string, maxSpeedMbps float64) error {
	// Create a temporary directory for segments
	tempDir, err := os.MkdirTemp("", "hls_download_*")
	if err != nil {
		return fmt.Errorf("could not create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Convert Mbps to bytes per second
	maxBytesPerSecond := 0
	if maxSpeedMbps > 0 {
		maxBytesPerSecond = int(maxSpeedMbps * 1000000 / 8)
		fmt.Printf("Using TokenBucket rate limiter: %d bytes/sec (%.1f Mbps)\n", maxBytesPerSecond, maxSpeedMbps)
	}

	// Download the master playlist first
	fmt.Println("Downloading HLS master playlist...")
	masterPlaylistPath := filepath.Join(tempDir, "master.m3u8")
	masterPlaylist, err := downloadFileWithTokenBucket(hlsUrl, masterPlaylistPath, maxBytesPerSecond)
	if err != nil {
		return fmt.Errorf("could not download master playlist: %w", err)
	}

	// Check if this is a master playlist or a direct media playlist
	bestQualityUrl, isMaster, err := getBestQualityPlaylist(masterPlaylist, hlsUrl)
	if err != nil {
		return fmt.Errorf("could not parse playlist: %w", err)
	}

	var mediaPlaylist string
	var mediaPlaylistUrl string

	if isMaster {
		// Download the best quality media playlist
		fmt.Printf("Downloading media playlist for best quality: %s\n", bestQualityUrl)
		mediaPlaylistPath := filepath.Join(tempDir, "media.m3u8")
		mediaPlaylist, err = downloadFileWithTokenBucket(bestQualityUrl, mediaPlaylistPath, maxBytesPerSecond)
		if err != nil {
			return fmt.Errorf("could not download media playlist: %w", err)
		}
		mediaPlaylistUrl = bestQualityUrl
	} else {
		// This is already a media playlist
		mediaPlaylist = masterPlaylist
		mediaPlaylistUrl = hlsUrl
	}

	// Parse the media playlist and download segments with rate limiting
	segmentUrls, err := parsePlaylist(mediaPlaylist, mediaPlaylistUrl)
	if err != nil {
		return fmt.Errorf("could not parse media playlist: %w", err)
	}

	if len(segmentUrls) == 0 {
		return fmt.Errorf("no segments found in media playlist")
	}

	fmt.Printf("Found %d segments to download\n", len(segmentUrls))

	// Download all segments with your token bucket rate limiting
	err = downloadSegmentsWithTokenBucket(segmentUrls, tempDir, maxBytesPerSecond)
	if err != nil {
		return fmt.Errorf("could not download segments: %w", err)
	}

	// Create a local playlist file pointing to downloaded segments
	localPlaylistPath := filepath.Join(tempDir, "local_playlist.m3u8")
	err = createLocalPlaylist(mediaPlaylist, localPlaylistPath, tempDir)
	if err != nil {
		return fmt.Errorf("could not create local playlist: %w", err)
	}

	// Now use ffmpeg to convert the local segments to final video (no network involved)
	fmt.Println("Converting segments to final video...")
	cmd := exec.Command(ffmpegCmd,
		"-i", localPlaylistPath,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		"-y", outputPath)

	// Capture stderr for debugging if needed
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadFileWithTokenBucket(url, outputPath string, maxBytesPerSecond int) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Use your existing TokenBucketRateLimitedReader
	var reader io.Reader = resp.Body
	if maxBytesPerSecond > 0 {
		rateLimitedReader := NewTokenBucketRateLimitedReader(resp.Body, maxBytesPerSecond)
		defer rateLimitedReader.Close()
		reader = rateLimitedReader
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	_, err = file.Write(content)
	return string(content), err
}

func getBestQualityPlaylist(playlist, baseUrl string) (string, bool, error) {
	lines := strings.Split(playlist, "\n")

	// Check if this is a master playlist by looking for #EXT-X-STREAM-INF
	isMaster := false
	var bestBandwidth int
	var bestUrl string

	// Extract base URL for relative paths
	baseUrlParts := strings.Split(baseUrl, "/")
	baseUrlParts = baseUrlParts[:len(baseUrlParts)-1] // Remove filename
	baseUrlPrefix := strings.Join(baseUrlParts, "/")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			isMaster = true
			// Extract bandwidth
			bandwidth := 0
			if strings.Contains(line, "BANDWIDTH=") {
				parts := strings.Split(line, "BANDWIDTH=")
				if len(parts) > 1 {
					bandwidthStr := strings.Split(parts[1], ",")[0]
					fmt.Sscanf(bandwidthStr, "%d", &bandwidth)
				}
			}

			// Get the URL from the next line
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if nextLine != "" && !strings.HasPrefix(nextLine, "#") {
					url := nextLine
					if !strings.HasPrefix(url, "http") {
						url = baseUrlPrefix + "/" + url
					}

					// Select the highest bandwidth (best quality)
					if bandwidth > bestBandwidth {
						bestBandwidth = bandwidth
						bestUrl = url
					}
				}
			}
		}
	}

	if isMaster {
		if bestUrl == "" {
			return "", true, fmt.Errorf("no valid stream found in master playlist")
		}
		fmt.Printf("Selected best quality stream with bandwidth: %d\n", bestBandwidth)
		return bestUrl, true, nil
	}

	// Not a master playlist, return the original URL
	return baseUrl, false, nil
}

func downloadSegmentsWithTokenBucket(urls []string, tempDir string, maxBytesPerSecond int) error {
	if maxBytesPerSecond > 0 {
		fmt.Printf("Downloading %d segments with TokenBucket rate limit %d bytes/sec...\n", len(urls), maxBytesPerSecond)
	} else {
		fmt.Printf("Downloading %d segments with no rate limit...\n", len(urls))
	}

	for i, segmentUrl := range urls {
		// Extract the original filename from the URL
		urlParts := strings.Split(segmentUrl, "/")
		originalFilename := urlParts[len(urlParts)-1]

		// Remove any query parameters
		if idx := strings.Index(originalFilename, "?"); idx != -1 {
			originalFilename = originalFilename[:idx]
		}

		// Use the original filename instead of generic segment_XXXX.ts
		segmentPath := filepath.Join(tempDir, originalFilename)

		resp, err := http.Get(segmentUrl)
		if err != nil {
			return fmt.Errorf("could not download segment %d (%s): %w", i, segmentUrl, err)
		}

		file, err := os.Create(segmentPath)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("could not create segment file %d: %w", i, err)
		}

		// Use your existing TokenBucketRateLimitedReader
		var reader io.Reader = resp.Body
		var rateLimitedReader *TokenBucketRateLimitedReader
		if maxBytesPerSecond > 0 {
			rateLimitedReader = NewTokenBucketRateLimitedReader(resp.Body, maxBytesPerSecond)
			reader = rateLimitedReader
		}

		_, err = io.Copy(file, reader)

		// Clean up
		if rateLimitedReader != nil {
			rateLimitedReader.Close()
		}
		file.Close()
		resp.Body.Close()

		if err != nil {
			return fmt.Errorf("could not write segment %d: %w", i, err)
		}

		// Progress indicator
		if (i+1)%10 == 0 || i == len(urls)-1 {
			fmt.Printf("Downloaded %d/%d segments\n", i+1, len(urls))
		}
	}

	fmt.Println("All segments downloaded successfully")
	return nil
}

func parsePlaylist(playlist, baseUrl string) ([]string, error) {
	lines := strings.Split(playlist, "\n")
	var urls []string

	// Extract base URL for relative paths
	baseUrlParts := strings.Split(baseUrl, "/")
	baseUrlParts = baseUrlParts[:len(baseUrlParts)-1] // Remove filename
	baseUrlPrefix := strings.Join(baseUrlParts, "/")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Convert relative URLs to absolute
			if strings.HasPrefix(line, "http") {
				urls = append(urls, line)
			} else {
				// Construct absolute URL
				absoluteUrl := baseUrlPrefix + "/" + line
				urls = append(urls, absoluteUrl)
			}
		}
	}

	return urls, nil
}

func createLocalPlaylist(originalPlaylist, localPlaylistPath, segmentDir string) error {
	lines := strings.Split(originalPlaylist, "\n")
	var newLines []string

	fmt.Println("Creating local playlist...")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Extract just the filename from the URL
			urlParts := strings.Split(line, "/")
			filename := urlParts[len(urlParts)-1]

			// Remove any query parameters
			if idx := strings.Index(filename, "?"); idx != -1 {
				filename = filename[:idx]
			}

			// Verify the file exists
			fullPath := filepath.Join(segmentDir, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				fmt.Printf("WARNING: Local file does not exist: %s\n", fullPath)
			}

			// Use the original filename instead of generic segment names
			newLines = append(newLines, filename)
		} else {
			newLines = append(newLines, line)
		}
	}

	content := strings.Join(newLines, "\n")
	return os.WriteFile(localPlaylistPath, []byte(content), 0644)
}
