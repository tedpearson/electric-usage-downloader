package app

import (
	"encoding/csv"
	"os"
	"strconv"
)

func WriteCsv(records []ElectricUsage, csvFile string) error {
	// Open the CSV file for writing
	file, err := os.Create(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write comments
	_, err = file.WriteString(`# Notes:
# MeterName will only be populated if there are multiple meters returned in the data.
# StartUnixMillis and EndUnixMillis will only be in the correct timezone if you specify the correct timezone in the config.
`)
	if err != nil {
		return err
	}

	// Write the header row
	header := []string{"StartUnixMillis", "EndUnixMillis", "WattHours", "CostInCents", "MeterName"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write the data rows
	for _, record := range records {
		meterName := ""
		if record.MeterName != nil {
			meterName = *record.MeterName
		}
		costInCents := ""
		if record.CostInCents != nil {
			costInCents = strconv.FormatInt(*record.CostInCents, 10)
		}
		row := []string{
			strconv.FormatInt(record.StartTime.UnixMilli(), 10),
			strconv.FormatInt(record.EndTime.UnixMilli(), 10),
			strconv.FormatInt(record.WattHours, 10),
			costInCents,
			meterName,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return nil
}
