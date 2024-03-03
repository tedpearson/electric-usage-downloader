package main

import (
	"electric-usage-downloader/internal/app"
)

func main() {
	if err := app.Main(); err != nil {
		panic(err)
	}
}
