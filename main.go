package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"electric-usage-downloader/internal/app"
)

var (
	startDate = "09/29/2022"
	endDate   = "09/30/2022"
)

func main() {
	// read config
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	config := &app.Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		log.Fatal(err)
	}

	path, err := app.DownloadCsv(config, startDate, endDate)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("file downloaded: %s", path)
	// parse csv!
	records, err := app.ParseCsv(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v", records)
}
