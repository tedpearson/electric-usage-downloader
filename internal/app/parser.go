package app

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type ElectricUsage struct {
	StartTime   time.Time
	EndTime     time.Time
	WattHours   int64
	CostInCents int64
}

type ElectricRecords struct {
	ElectricUsage []*ElectricUsage
}

func ParseCsv(file string) (*ElectricRecords, error) {
	dateRegex, err := regexp.Compile(`(\d{4}-\d\d-\d\d \d\d:\d\d) to (\d{4}-\d\d-\d\d \d\d:\d\d)`)
	if err != nil {
		log.Fatal(err)
	}
	// drop all lines until it starts with dddd-dd-dd
	data, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := data.Close(); err != nil {
			panic(err)
		}
	}()
	scanner := bufio.NewScanner(data)

	records := make([]*ElectricUsage, 0, 100)
	for scanner.Scan() {
		line := scanner.Text()
		matches := dateRegex.FindStringSubmatch(line)
		if matches != nil {
			cols := strings.Split(line, ",")
			usage := parseRecord(cols, matches[1:3])
			records = append(records, usage)
		}
	}
	return &ElectricRecords{records}, nil
}

func parseRecord(row []string, dates []string) *ElectricUsage {
	if len(row) < 3 {
		log.Printf("Bad record: %s\n", row)
		return nil
	}
	startTime, err1 := time.ParseInLocation("2006-01-02 15:04", dates[0], time.Local)
	endTime, err2 := time.ParseInLocation("2006-01-02 15:04", dates[1], time.Local)
	kilowattHours, err3 := decimal.NewFromString(row[1])
	cents, err4 := decimal.NewFromString(row[2])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		log.Printf("Bad record: %s\n", row)
		return nil
	}
	return &ElectricUsage{
		StartTime:   startTime,
		EndTime:     endTime,
		WattHours:   kilowattHours.Mul(decimal.NewFromInt(1000)).IntPart(),
		CostInCents: cents.Mul(decimal.NewFromInt(100)).IntPart(),
	}
}
