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
	configFlag := flag.String("config", "config.yaml", "Config file")
	startFlag := flag.String("start", "", "Start date of period to extract from electric co.")
	endFlag := flag.String("end", "", "End date of period to extract from electric co.")
	flag.Parse()

	// read config
	file, err := os.ReadFile(*configFlag)
	if err != nil {
		log.Fatal(err)
	}
	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		log.Fatal(err)
	}
	if config.ExtractDays > 45 || config.ExtractDays < 2 {
		log.Fatal("ExtractDays must be between 2 and 45 per smarthub")
	}

	var startDate, endDate time.Time
	if *startFlag != "" {
		startDate, err = time.ParseInLocation("2006-01-02", *startFlag, time.Local)
		if err != nil {
			log.Fatal(err)
		}
		if *endFlag == "" {
			log.Fatal("start and end parameters must both be provided")
		}
		endDate, err = time.ParseInLocation("2006-01-02", *endFlag, time.Local)
		if err != nil {
			log.Fatal(err)
		}
		if endDate.Sub(startDate).Hours() > 24*45 {
			log.Fatal("start and end parameters must define a period of no more than 45 days")
		}
		// endDate should be the last minute of the day for the VictoriaMetrics query.
		endDate = endDate.Add((24 * time.Hour) - time.Minute)
	} else {
		// yesterday
		year, month, day := time.Now().Date()
		// endDate should be the last minute of the day for the VictoriaMetrics query.
		endDate = time.Date(year, month, day, 23, 59, 0, 0, time.Local)
		// subtract N days and 1 minute to get the start date
		startDate = endDate.Add(time.Duration(-config.ExtractDays) * 48 * time.Hour).Add(time.Minute)
	}

	path, err := DownloadCsv(config, startDate.Format("01/02/2006"), endDate.Format("01/02/2006"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("file downloaded: %s", path)

	records, err := ParseCsv(path)
	if err != nil {
		log.Fatal(err)
	}
	existingPoints, err := QueryPreviousMetrics(startDate, endDate, config.InfluxDB)
	if err != nil {
		log.Fatal(err)
	}
	err = WriteMetrics(records, config.InfluxDB, existingPoints)
	if err != nil {
		log.Fatal(err)
	}
}
