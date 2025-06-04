package commons

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Action string

const (
	Exit     Action = "exit"
	Download Action = "download"
	Search   Action = "search"
	None     Action = "none"
)

var SupportedDomains = []string{
	"animesaturn.*",
}

type DownloadConfig struct {
	BatchSize    int     // number of max concurrent downloads
	MaxSpeedMbps float64 // maximum speed in Mbps
}

type SetupResult struct {
	Playwright       *playwright.Playwright
	Browser          playwright.Browser
	Page             playwright.Page
	URL              string
	Action           Action
	EpisodeRange     string // Format: "start-end"
	SpecificEpisodes string // Format: "1,3,5,7"
	IsHeadless       bool
	DownloadConfig   DownloadConfig
	FFmpegPath       string
}

func printHelp() {
	fmt.Println("OtakuCrawler - Anime Web Scraper")
	fmt.Println("Usage:")
	fmt.Println("  --link, -l <URL>     Specify the target URL to scrape")
	fmt.Println("  --download, -d       Download episodes from the URL")
	fmt.Println("  --search, -s         Get streaming links without downloading")
	fmt.Println("  --range, -r <X-Y>    Download only episodes X through Y")
	fmt.Println("  --only, -o <X,Y,Z>   Download only specific episodes X, Y, and Z")
	fmt.Println("  --batch, -b <N>      Number of concurrent downloads (default: 3)")
	fmt.Println("  --speed, -sp <N>     Maximum download speed in Mbps (default: 20.0)")
	fmt.Println("  --headless, -hl      Run browser in headless mode (no visible window, recommended)")
	fmt.Println("  --help, -h           Show this help message")
}

func downloadFFmpeg(appDir, destPath string) error {
	fmt.Println("Downloading FFmpeg... Please wait")

	// FFmpeg download is more complex as it comes in an archive
	var url string
	var archiveName string
	var executablePath string

	// Determine the URL and paths based on platform
	if runtime.GOOS == "windows" {
		// Example URL - you might need to update this with a more current version
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
		archiveName = filepath.Join(appDir, "ffmpeg.zip")
		executablePath = "ffmpeg-master-latest-win64-gpl/bin/ffmpeg.exe"
	} else if runtime.GOOS == "darwin" {
		url = "https://evermeet.cx/ffmpeg/getrelease/zip"
		archiveName = filepath.Join(appDir, "ffmpeg.zip")
		executablePath = "ffmpeg"
	} else {
		// Linux - you might need a different approach for Linux
		url = "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
		archiveName = filepath.Join(appDir, "ffmpeg.tar.xz")
		executablePath = "ffmpeg-*-amd64-static/ffmpeg"
	}

	// Download the archive
	fmt.Printf("Downloading from: %s\n", url)
	cmd := exec.Command("curl", "-L", url, "-o", archiveName)
	setWindowsCmdAttrs(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download FFmpeg: %w", err)
	}

	// Extract the archive
	fmt.Println("Extracting FFmpeg archive...")
	if strings.HasSuffix(archiveName, ".zip") {
		// Extract ZIP
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-Command",
				"Expand-Archive", "-Path", archiveName, "-DestinationPath", appDir, "-Force")
		} else {
			// Use unzip on Mac/Linux
			cmd = exec.Command("unzip", "-o", archiveName, "-d", appDir)
		}
		setWindowsCmdAttrs(cmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract FFmpeg archive: %w", err)
		}
	} else if strings.HasSuffix(archiveName, ".tar.xz") {
		// Extract tar.xz on Linux
		cmd = exec.Command("tar", "-xf", archiveName, "-C", appDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract FFmpeg archive: %w", err)
		}
	}

	// Find the extracted ffmpeg executable
	var ffmpegPath string
	if runtime.GOOS == "windows" {
		ffmpegPath = filepath.Join(appDir, executablePath)
	} else {
		// For Linux/Mac, might need to find the file
		matches, err := filepath.Glob(filepath.Join(appDir, executablePath))
		if err != nil || len(matches) == 0 {
			return fmt.Errorf("could not find extracted ffmpeg executable")
		}
		ffmpegPath = matches[0]
	}

	// Copy the executable to the destination
	input, err := os.Open(ffmpegPath)
	if err != nil {
		return fmt.Errorf("could not open source FFmpeg: %w", err)
	}
	defer input.Close()

	output, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create destination FFmpeg: %w", err)
	}
	defer output.Close()

	if _, err = io.Copy(output, input); err != nil {
		return fmt.Errorf("could not copy FFmpeg: %w", err)
	}

	// Make executable on non-Windows platforms
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("could not make FFmpeg executable: %w", err)
		}
	}

	// Clean up the archive and extracted files
	os.Remove(archiveName)
	if runtime.GOOS == "windows" {
		// Clean up extracted directory on Windows
		extractedDir := filepath.Join(appDir, "ffmpeg-master-latest-win64-gpl")
		os.RemoveAll(extractedDir)
	} else if runtime.GOOS == "linux" {
		// Clean up extracted directory on Linux
		matches, _ := filepath.Glob(filepath.Join(appDir, "ffmpeg-*-amd64-static"))
		for _, match := range matches {
			os.RemoveAll(match)
		}
	}

	fmt.Printf("Successfully installed FFmpeg to: %s\n", destPath)
	return nil
}

func setupFFmpeg() string {
	// First check if ffmpeg is already in PATH
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		fmt.Println("FFmpeg found in system PATH")
		return "ffmpeg" // Return the system ffmpeg
	}

	// Set up app directory for storing FFmpeg
	userDir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("Warning: Could not get user config dir: %v", err)
		userDir = os.TempDir()
	}

	appDir := filepath.Join(userDir, "OtakuCrawler")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Printf("Warning: Could not create app directory: %v", err)
		return ""
	}

	ffmpegPath := filepath.Join(appDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpegPath += ".exe"
	}

	// Check if FFmpeg already exists in our app directory
	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		fmt.Println("FFmpeg not found locally, downloading...")
		err = downloadFFmpeg(appDir, ffmpegPath)
		if err != nil {
			log.Printf("Warning: Could not download FFmpeg: %v", err)
			fmt.Println("⚠️  FFmpeg download failed. HLS streams will not be downloadable.")
			fmt.Println("💡 You can manually install FFmpeg and add it to your PATH")
			return ""
		}
	} else {
		fmt.Printf("Using local FFmpeg at: %s\n", ffmpegPath)
	}

	return ffmpegPath
}

func CommonSetup() SetupResult {
	var link string
	var action = None
	var episodeRange string
	var specificEpisodes string
	var isHeadless = false

	downloadConfig := DownloadConfig{
		BatchSize:    3,
		MaxSpeedMbps: 1000.0,
	}

	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		return SetupResult{Action: Exit}
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			printHelp()
			return SetupResult{Action: Exit}
		case "--link", "-l":
			if i+1 < len(args) {
				link = args[i+1]
				i++
			} else {
				log.Fatal("Error: --link requires a URL argument")
			}
		case "--download", "-d":
			if action != None {
				log.Fatal("Error: Multiple actions specified. Choose one: --download, --search, or --fetch.")
			}
			action = Download
		case "--search", "-s", "--fetch", "-f":
			if action != None {
				log.Fatal("Error: Multiple actions specified. Choose one: --download, --search, or --fetch.")
			}
			action = Search
		case "--range", "-r":
			if i+1 < len(args) {
				episodeRange = args[i+1]
				i++
			} else {
				log.Fatal("Error: --range requires a value in the format 'start-end'")
			}
		case "--only", "-o":
			if i+1 < len(args) {
				specificEpisodes = args[i+1]
				i++
			} else {
				log.Fatal("Error: --only requires a comma-separated list of episode numbers")
			}
		case "--batch", "-b":
			if i+1 < len(args) {
				batchSize, err := strconv.Atoi(args[i+1])
				if err != nil || batchSize < 1 {
					log.Fatal("Error: --batch requires a positive integer")
				}
				downloadConfig.BatchSize = batchSize
				i++
			} else {
				log.Fatal("Error: --batch requires a positive integer argument")
			}
		case "--speed", "-sp":
			if i+1 < len(args) {
				speed, err := strconv.ParseFloat(args[i+1], 64)
				if err != nil || speed <= 0 {
					log.Fatal("Error: --speed requires a positive number")
				}
				downloadConfig.MaxSpeedMbps = speed
				i++
			} else {
				log.Fatal("Error: --speed requires a positive number argument")
			}
		case "--headless", "-hl":
			isHeadless = true
		default:
			log.Fatalf("Unknown argument: %s\nUse --help to see usage.", args[i])
		}
	}

	// Make sure both --range and --only are not specified together
	if episodeRange != "" && specificEpisodes != "" {
		log.Fatal("Error: Cannot use both --range and --only at the same time")
	}

	if link == "" {
		log.Fatal("Error: No link provided. Use --link or -l followed by a URL.")
	}

	if !isSupportedLink(link) {
		log.Fatalf("Error: Link is not from a supported domain.\nSupported domains: %v", SupportedDomains)
	}

	fmt.Printf("Action: %s, URL: %s\n", action, link)
	if action == Download {
		fmt.Printf("Download Config: Batch Size: %d, Max Speed: %.1f Mbps\n",
			downloadConfig.BatchSize, downloadConfig.MaxSpeedMbps)
	}

	if action == None {
		os.Exit(0)
	}

	// Install dependencies (Playwright and FFmpeg)
	if !installDeps() {
		return SetupResult{Action: Exit}
	}

	// Setup FFmpeg
	ffmpegPath := setupFFmpeg()

	pw, err := playwright.Run(&playwright.RunOptions{Browsers: []string{"firefox"}})
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}

	browser, err := pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(isHeadless),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	if _, err = page.Goto(link); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	return SetupResult{
		Playwright:       pw,
		Browser:          browser,
		Page:             page,
		URL:              link,
		Action:           action,
		EpisodeRange:     episodeRange,
		SpecificEpisodes: specificEpisodes,
		IsHeadless:       isHeadless,
		DownloadConfig:   downloadConfig,
		FFmpegPath:       ffmpegPath,
	}
}

func isSupportedLink(link string) bool {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(parsedURL.Hostname(), "www.")

	for _, domain := range SupportedDomains {
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

func installDeps() bool {
	fmt.Println("Installing dependencies.. Please wait")
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"firefox"},
	})
	if err != nil {
		log.Fatalf("could not install playwright: %v", err)
	}

	if runtime.GOOS == "windows" {
		checkDLL := func(name string) bool {
			_, err := exec.LookPath(name)
			return err == nil
		}

		requiredDLLs := []string{"mf.dll", "mfplat.dll"}
		var missing []string

		for _, dll := range requiredDLLs {
			if !checkDLL(dll) {
				missing = append(missing, dll)
			}
		}

		if len(missing) > 0 {
			fmt.Println("\n⚠️  Missing Media Foundation components:")
			for _, dll := range missing {
				fmt.Printf(" - %s\n", dll)
			}
			fmt.Println("\nPlease install the Media Feature Pack:")
			fmt.Println("🔗 https://support.microsoft.com/en-us/help/3145500/media-feature-pack-list-for-windows-n-editions")

			fmt.Println("\nIf you're on Windows Server, run this as Administrator in PowerShell:")
			fmt.Println("  Install-WindowsFeature Server-Media-Foundation")
			return false
		}
	}

	fmt.Println("Successfully installed dependencies")
	return true
}
