package main

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

type Action string

const (
	Exit     Action = "exit"
	Download Action = "download"
	Search   Action = "search"
	None     Action = "none"
)

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
	fmt.Println("  --headless           Run browser in headless mode (no visible window, recommended)")
	fmt.Println("  --help, -h           Show this help message")
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
		case "--headless":
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
		log.Fatalf("Error: Link is not from a supported domain.\nSupported domains: %v", supportedDomains)
	}

	fmt.Printf("Action: %s, URL: %s\n", action, link)
	if action == Download {
		fmt.Printf("Download Config: Batch Size: %d, Max Speed: %.1f Mbps\n",
			downloadConfig.BatchSize, downloadConfig.MaxSpeedMbps)
	}

	if action == None {
		os.Exit(0)
	}

	if !installDeps() {
		return SetupResult{Action: Exit}
	}

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
	}
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
