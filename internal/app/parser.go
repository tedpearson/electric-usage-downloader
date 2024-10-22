package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ElectricUsage contains usage and cost information for a defined period of time.
type ElectricUsage struct {
	StartTime   time.Time
	EndTime     time.Time
	WattHours   int64
	CostInCents *int64
	MeterName   *string
}

// Response holds the parsed response from the SmartHub poll api.
// If Status is "PENDING", this means we need to make the same request again
// as data is still being prepared.
type Response struct {
	Status string                    `json:"status"`
	Data   map[string][]SmartHubData `json:"data"`
}

// SmartHubData holds parsed response data from the SmartHub poll api.
// It holds the Type of data ("USAGE" or "COST"), and the Series
type SmartHubData struct {
	Type   string           `json:"type"`
	Series []SmartHubSeries `json:"series"`
}

// SmartHubSeries holds parsed response data from the SmartHub poll api.
// It holds a list of SmartHubPoints.
type SmartHubSeries struct {
	Name string          `json:"name"`
	Data []SmartHubPoint `json:"data"`
}

// SmartHubPoint  holds parsed response data from the SmartHub poll api.
// It holds a timestamp, UnixMillis, which is actually in the America/New_York timezone instead of
// UTC as it should be.
// It also holds the Value of the point in dollars or kWh.
type SmartHubPoint struct {
	UnixMillis int64   `json:"x"`
	Value      float64 `json:"y"`
}

// RetryableError is an error that indicates to the retryer in Main that another
// poll request to FetchData should be made.
type RetryableError struct {
	Msg string
}

// NewRetryableError creates a RetryableError
func NewRetryableError(msg string) *RetryableError {
	return &RetryableError{Msg: msg}
}

// Error implements type error for RetryableError.
func (t *RetryableError) Error() string {
	return t.Msg
}

// ParseReader parses the json response received in FetchData from the SmartHub poll api.
// It can return a normal error, a RetryableError, or parsed ElectricUsage.
func ParseReader(reader *bytes.Reader, timezone string) ([]ElectricUsage, error) {
	if debug {
		_, _ = fmt.Fprintln(os.Stderr, "\nDEBUG: Response from poll endpoint:")
		_, err := reader.WriteTo(os.Stderr)
		if err != nil {
			return nil, err
		}
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	resp := &Response{}
	err := json.NewDecoder(reader).Decode(resp)
	if err != nil {
		return nil, err
	}
	if debug {
		_, _ = fmt.Fprintln(os.Stderr, "\n\nDEBUG: Parsed data from poll endpoint:")
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n\n", resp)
	}
	if resp.Status != "COMPLETE" {
		fmt.Println("Data not ready, retrying...")
		return nil, NewRetryableError("data processing not complete")
	}
	fmt.Println("Data received, transforming...")
	datas, ok := resp.Data["ELECTRIC"]
	if !ok {
		return nil, errors.New("no ELECTRIC key")
	}
	var usageData, costData []SmartHubSeries
	for _, data := range datas {
		switch data.Type {
		case "USAGE":
			usageData = data.Series
		case "COST":
			costData = data.Series
		}
	}
	// this is dumb, but the SmartHub api returns "unix timestamps"
	// that are based on the utility timezone (which is incorrect), at least as of 2/29/2024.
	// Example: For Midnight, Jan 1, 1970, EST, this api would return "0"
	//          However, the correct value (UTC) would be "18000".
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	_, offset := time.Now().In(zone).Zone()

	seriesCount := len(usageData)
	dataCount := len(usageData[0].Data)
	records := make([]ElectricUsage, seriesCount*dataCount)
	for i := range usageData {
		meterName := parseName(usageData[i].Name, seriesCount)
		usageSeries := usageData[i].Data
		period := time.UnixMilli(usageSeries[1].UnixMillis).Sub(time.UnixMilli(usageSeries[0].UnixMillis))
		for j := range usageSeries {
			usage := usageSeries[j]
			index := j + (i * dataCount)
			// see note above about "unix timestamps"
			start := time.UnixMilli(usage.UnixMillis).Add(time.Second * time.Duration(-offset))
			records[index].StartTime = start
			records[index].EndTime = start.Add(period)
			records[index].WattHours = int64(usage.Value * 1000)
			records[index].MeterName = meterName
		}
		// note: cost is not returned by all SmartHub implementations.
		if len(costData) > i {
			costSeries := costData[i].Data
			for j := range costSeries {
				cost := int64(costSeries[j].Value * 100)
				records[j+(i*dataCount)].CostInCents = &cost
			}
		}
	}
	return records, nil
}

// parseName returns the last part of a string after a space. If seriesCount is 1, returns nil.
func parseName(name string, seriesCount int) *string {
	var result *string
	if seriesCount > 1 {
		tokens := strings.Split(name, " ")
		result = &tokens[len(tokens)-1]
	}
	return result
}
