[![forthebadge](https://forthebadge.com/images/badges/made-with-go.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/built-with-love.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/you-didnt-ask-for-this.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/60-percent-of-the-time-works-every-time.svg)](https://forthebadge.com)

# OtakuCrawler

A powerful tool for batch downloading anime episodes from supported websites.

## Features

- Batch processing of episodes
- Select specific episodes or ranges
- Headless mode for server environments
- Automatic file existence detection
- Multi-threaded downloads

## Installation

### Option 1: Download Pre-built Binary (Recommended)
[![Latest Release](https://img.shields.io/github/v/release/milkyicedtea/OtakuCrawler)](https://github.com/milkyicedtea/OtakuCrawler/releases/latest)

1. Go to the [Releases page](https://github.com/milkyicedtea/OtakuCrawler/releases)
2. Download the appropriate version for your operating system:
   - Windows: `otakucrawler-windows-amd64.exe`
   - macOS (Intel): `otakucrawler-darwin-amd64`
   - macOS (Apple Silicon): `otakucrawler-darwin-arm64`
   - Linux: `otakucrawler-linux-amd64`
3. Might need to make the file executable (Optional, Linux/macOS only):
```bash
chmod +x otakucrawler-*
```

### Option 2: Build from source
```bash
# Clone the repository
git clone https://github.com/milkyicedtea/OtakuCrawler
cd OtakuCrawler

# Build the application
go build
```

## Usage
```bash
# Download all episodes from an anime
./otakucrawler --link https://examplesite.com/anime/example --download

# Download specific episodes
./otakucrawler --link https://examplesite.com/anime/example --download --only 1,3,5

# Download a range of episodes
./otakucrawler --link https://examplesite.com/anime/example --download --range 5-10

# Run in headless mode (no browser window, recommended)
./otakucrawler --link https://examplesite.com/anime/example --download --headless

# Get just the streaming links without downloading (if you have an external downloader)
./otakucrawler --link https://examplesite.com/anime/example --search
```

## Supported Sites
- AnimeSaturn
- More to come.. soon™ :)

## License
This software is released under a custom non-commercial license. See the [LICENSE](LICENSE.md) file for more details
