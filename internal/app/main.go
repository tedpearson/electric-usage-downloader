package app

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

//var (
//	startDate = "09/29/2022"
//	endDate   = "09/30/2022"
//)

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
	ExtractDays int
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

	// parse time flags
	startFlag := flag.String("start", "", "Start date of period to extract from electric co.")
	endFlag := flag.String("end", "", "End date of period to extract from electric co.")
	flag.Parse()
	var startDate, endDate time.Time
	if *startFlag != "" {
		startDate, err = time.Parse("2006-01-02", *startFlag)
		if err != nil {
			log.Fatal(err)
		}
		if *endFlag == "" {
			log.Fatal("start and end parameters must both be provided")
		}
		endDate, err = time.Parse("2006-01-02", *endFlag)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		endDate = time.Now().Truncate(24 * time.Hour)
		startDate = endDate.Add(time.Duration(-config.ExtractDays) * 24 * time.Hour)
	}

	path, err := DownloadCsv(config, startDate.Format("01/02/2006"), endDate.Format("01/02/2002"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("file downloaded: %s", path)

	// parse csv
	records, err := ParseCsv(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v", records)
	existingPoints, err := QueryPreviousMetrics(startDate, endDate, config.InfluxDB)
	if err != nil {
		log.Fatal(err)
	}
	WriteMetrics(records, config.InfluxDB, existingPoints)
}
