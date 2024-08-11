package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"gopkg.in/yaml.v3"
)

// InfluxConfig is the config for the VictoriaMetrics connection, via the influxdb client.
type InfluxConfig struct {
	Host      string `yaml:"host"`
	AuthToken string `yaml:"auth_token"`
	Org       string `yaml:"org"`
	Database  string `yaml:"database"`
	Insecure  bool   `yaml:"insecure"`
}

// SmartHubConfig is the config for SmartHub.
// Account is your account number, available on your bill.
// ServiceLocation appears to be an internal number, and must be retrieved from your browser. See README.md.
type SmartHubConfig struct {
	ApiUrl          string `yaml:"api_url"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Account         string `yaml:"account"`
	ServiceLocation string `yaml:"service_location"`
	Timezone        string `yaml:"timezone"`
}

// Config is the config format for electric-usage-downloader
type Config struct {
	ExtractDays int            `yaml:"extract_days"`
	SmartHub    SmartHubConfig `yaml:"smarthub"`
	InfluxDB    InfluxConfig   `yaml:"influxdb"`
}

var debug bool

// Main runs the program.
func Main() error {
	configFlag := flag.String("config", "config.yaml", "Config file")
	startFlag := flag.String("start", "", "Start date of period to extract from electric co.")
	endFlag := flag.String("end", "", "End date of period to extract from electric co.")
	debugFlag := flag.Bool("debug", false, "Enable to print out verbose debugging logs.")
	flag.Parse()

	debug = *debugFlag
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
	fmt.Printf("Start date: %s\n", startDate)
	fmt.Printf("End date: %s\n", endDate)

	fmt.Println("Authenticating with SmartHub API...")
	jwt, err := Auth(config.SmartHub)
	if err != nil {
		return err
	}

	fmt.Println("Fetching data from SmartHub API...")
	usage, err := retry.DoWithData(
		func() ([]ElectricUsage, error) {
			r, err := FetchData(startDate, endDate, config.SmartHub, jwt)
			if err != nil {
				return nil, err
			}
			records, err := ParseReader(r, config.SmartHub.Timezone)
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
	fmt.Println("Done")
	return nil
}
