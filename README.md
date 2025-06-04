[![forthebadge](https://forthebadge.com/images/badges/made-with-go.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/built-with-love.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/you-didnt-ask-for-this.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/60-percent-of-the-time-works-every-time.svg)](https://forthebadge.com)

# OtakuCrawler

A powerful tool for batch downloading anime episodes from supported websites.

## Features
- Batch processing with configurable concurrent downloads
- Speed limiting to control bandwidth usage
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

### Basic Commands
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

### Advanced Download Control
```bash
# Control concurrent downloads (default: 3)
./otakucrawler --link https://examplesite.com/anime --download --batch 4

# Limit download speed (in Mbps, default: 0 = no limit)
./otakucrawler --link https://examplesite.com/anime --download --speed 10

# Combine batch size and speed limiting
./otakucrawler --link https://examplesite.com/anime --download --batch 2 --speed 20

# Download episodes 1-5 with 4 concurrent downloads at 15 Mbps max
./otakucrawler --link https://examplesite.com/anime --download --range 1-5 --batch 4 --speed 15
```

### Command Line Options
| Option       | Short | Description                                   | Default      |
|--------------|-------|-----------------------------------------------|--------------|
| `--link`     | `-l`  | Target URL to scrape                          | Required     |
| `--download` | `-d`  | Download episodes from the URL                |              |
| `--search`   | `-s`  | Get streaming links without downloading       |              |
| `--range`    | `-r`  | Download episodes X through Y (format: X-Y)   | All episodes |
| `--only`     | `-o`  | Download specific episodes (format: X,Y,Z)    | All episodes |
| `--batch`    | `-b`  | Number of concurrent downloads                | 3            |
| `--speed`    | `-sp` | Maximum download speed in Mbps (0 = no limit) | 0            |
| `--headless` | `-hl` | Run browser in headless mode                  | false        |
| `--help`     | `-h`  | Show help message                             |              |

### Examples
> [!Note]
> These examples all include `--headless` because although not the default option,
> i think it's the best way to use this tool.
> All of these will obviously still work even without it.
```bash
# Conservative setup: 2 concurrent downloads at 5 Mbps each (10 Mbps total)
./otakucrawler -l https://examplesite.com/anime/example -d -b 2 -sp 10 --headless

# Aggressive setup: 6 concurrent downloads with no speed limit
./otakucrawler -l https://examplesite.com/anime/example -d -b 6 -sp 0 --headless

# Download specific episodes with moderate settings
./otakucrawler -l https://examplesite.com/anime/example -d --only 1,5,10,15 -b 3 -sp 25 --headless

# Download latest 5 episodes quickly
./otakucrawler -l https://examplesite.com/anime/example -d --range 20-24 -b 4 --headless
```

## Performance Notes
- **Batch Size**: Higher values = faster overall completion but more resource usage
- **Speed Limiting**: Set based on your internet connection and usage needs
  - Set to 0 for maximum speed on unlimited connections
- **Headless Mode**: Recommended for better performance and server environments

## Supported Sites
- AnimeSaturn
- More to come.. soon™ :)

## License
This software is released under a custom non-commercial license. See the [LICENSE](LICENSE.md) file for more details
