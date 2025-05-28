package main

import (
	"fmt"
)

var (
	Version   = "1.0.0"
	BuildDate = "2025-05-28"
)

func printBanner() {
	fmt.Printf("  ___    _             _                ____                             _\n / _ \\  | |_    __ _  | | __  _   _    / ___|  _ __    __ _  __      __ | |   ___   _ __\n| | | | | __|  / _` | | |/ / | | | |  | |     | '__|  / _` | \\ \\ /\\ / / | |  / _ \\ | '__|\n| |_| | | |_  | (_| | |   <  | |_| |  | |___  | |    | (_| |  \\ V  V /  | | |  __/ | |\n \\___/   \\__|  \\__,_| |_|\\_\\  \\__,_|   \\____| |_|     \\__,_|   \\_/\\_/   |_|  \\___| |_|\n")

	fmt.Printf("Version: %s (Build: %s)\n", Version, BuildDate)
	fmt.Println("Created by: Cheek/milkyicedtea")
}
