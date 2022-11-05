package main

import (
	"electric-usage-downloader/internal/app"
	"log"
)

func main() {
	if err := app.Main(); err != nil {
		log.Fatal(err)
	}
}
