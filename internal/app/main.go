package app

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"gopkg.in/yaml.v3"
)

// InfluxConfig is the config for the VictoriaMetrics connection, via the influxdb client.
type InfluxConfig struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Insecure bool   `yaml:"insecure"`
}

// UtilityConfig is the config for Novec.
// Password is hashed or encrypted in some unknown way, and must be retrieved from your browser. (TBD)
// Account is your account number, available on your bill.
// ServiceLocation appears to be an internal number, and must be retrieved from your browser. (TBD)
type UtilityConfig struct {
	ApiUrl          string `yaml:"api_url"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Account         string `yaml:"account"`
	ServiceLocation string `yaml:"service_location"`
}

// Config is the config format for electric-usage-downloader
type Config struct {
	ExtractDays int           `yaml:"extract_days"`
	Utility     UtilityConfig `yaml:"utility"`
	InfluxDB    InfluxConfig  `yaml:"influxdb"`
}

// Main runs the program.
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

	log.Println("Authenticating with Novec API...")
	jwt, err := Auth(config.Utility)
	if err != nil {
		return err
	}

	log.Println("Fetching data from Novec API...")
	usage, err := retry.DoWithData(
		func() ([]ElectricUsage, error) {
			r, err := FetchData(startDate, endDate, config.Utility, jwt)
			if err != nil {
				return nil, err
			}
			records, err := ParseReader(r)
			if err != nil {
				return nil, err
			}
			return records, nil
		}, retry.RetryIf(func(err error) bool {
			var retryableError *RetryableError
			return errors.As(err, &retryableError)
		}), retry.Delay(time.Second), retry.Attempts(10))

	if err != nil {
		return err
	}
	fmt.Println("Writing data to database...")
	err = WriteMetrics(usage, config.InfluxDB)
	if err != nil {
		return err
	}
	log.Println("Done")
	return nil
}
