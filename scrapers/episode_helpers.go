package scrapers

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseEpisodeRange parses a string like "3-7" into start and end indices
func ParseEpisodeRange(rangeStr string) (int, int, error) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format, expected 'start-end'")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start value: %w", err)
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end value: %w", err)
	}

	if start < 1 || end < start {
		return 0, 0, fmt.Errorf("invalid range: start must be ≥ 1 and end must be ≥ start")
	}

	// Convert to 0-based indices for internal use
	return start - 1, end - 1, nil
}

// ParseSpecificEpisodes parses a string like "1,3,5" into a slice of indices
func ParseSpecificEpisodes(episodesStr string) ([]int, error) {
	parts := strings.Split(episodesStr, ",")
	episodes := make([]int, 0, len(parts))

	for _, part := range parts {
		ep, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid episode number '%s': %w", part, err)
		}

		if ep < 1 {
			return nil, fmt.Errorf("episode numbers must be ≥ 1")
		}

		// Convert to 0-based index for internal use
		episodes = append(episodes, ep-1)
	}

	return episodes, nil
}

// ShouldProcessEpisode checks if an episode should be processed based on range or specific episodes
func ShouldProcessEpisode(idx int, start, end int, specificEpisodes []int) bool {
	if start >= 0 && end >= 0 {
		return idx >= start && idx <= end
	}

	if specificEpisodes != nil {
		for _, ep := range specificEpisodes {
			if idx == ep {
				return true
			}
		}
		return false
	}

	// If no filters are specified, process all episodes
	return true
}
