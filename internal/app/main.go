package app

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	startDate = "09/29/2022"
	endDate   = "09/30/2022"
)

type InfluxDB struct {
	Host     string
	User     string
	Password string
	Database string
}

type Config struct {
	Username    string
	Password    string
	LoginUrl    string
	DownloadDir string
	InfluxDB    InfluxDB
}

func Main() {

	// read config
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		log.Fatal(err)
	}

	path, err := DownloadCsv(config, startDate, endDate)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("file downloaded: %s", path)
	// parse csv!
	records, err := ParseCsv(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v", records)
	WriteMetrics(records, config.InfluxDB)
}
