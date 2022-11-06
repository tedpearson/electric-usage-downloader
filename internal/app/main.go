package app

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
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
	ExtractDays int
	Timeout     string
	Headless    bool
	InfluxDB    InfluxDB
}

func Main() error {
	configFlag := flag.String("config", "config.yaml", "Config file")
	startFlag := flag.String("start", "", "Start date of period to extract from electric co.")
	endFlag := flag.String("end", "", "End date of period to extract from electric co.")
	flag.Parse()

	// read config
	file, err := os.ReadFile(*configFlag)
	if err != nil {
		return err
	}
	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return err
	}
	if config.ExtractDays > 45 || config.ExtractDays < 2 {
		return errors.New("ExtractDays must be between 2 and 45 per smarthub")
	}

	var startDate, endDate time.Time
	if *startFlag != "" {
		startDate, err = time.ParseInLocation("2006-01-02", *startFlag, time.Local)
		if err != nil {
			return err
		}
		if *endFlag == "" {
			return errors.New("start and end parameters must both be provided")
		}
		endDate, err = time.ParseInLocation("2006-01-02", *endFlag, time.Local)
		if err != nil {
			return err
		}
		if endDate.Sub(startDate).Hours() > 24*45 {
			return errors.New("start and end parameters must define a period of no more than 45 days")
		}
		// endDate should be the last minute of the day for the VictoriaMetrics query.
		endDate = endDate.Add((24 * time.Hour) - time.Minute)
	} else {
		// yesterday
		year, month, day := time.Now().Date()
		// endDate should be the last minute of the day for the VictoriaMetrics query.
		endDate = time.Date(year, month, day, 23, 59, 0, 0, time.Local)
		// subtract N days and 1 minute to get the start date
		startDate = endDate.Add(time.Duration(-config.ExtractDays) * 24 * time.Hour).Add(time.Minute)
	}
	log.Printf("Start date: %s\n", startDate)
	log.Printf("End date: %s\n", endDate)
	log.Println("Downloading CSV...")
	path, err := DownloadCsv(config, startDate.Format("01/02/2006"), endDate.Format("01/02/2006"))
	if err != nil {
		return err
	}
	log.Printf("CSV downloaded: %s\n", path)
	defer cleanup(path)

	records, err := ParseCsv(path)
	if err != nil {
		return err
	}
	log.Println("Querying previous metrics...")
	existingPoints, err := QueryPreviousMetrics(startDate, endDate, config.InfluxDB)
	if err != nil {
		return err
	}
	log.Println("Inserting data...")
	err = WriteMetrics(records, config.InfluxDB, existingPoints)
	if err != nil {
		return err
	}
	log.Println("Done")
	return nil
}

func cleanup(path string) {
	log.Printf("Removing CSV: %s", path)
	if err := os.Remove(path); err != nil {
		log.Printf("Failed to remove CSV: %s", path)
	}
}
