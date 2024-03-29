package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// ElectricUsage contains usage and cost information for a defined period of time.
type ElectricUsage struct {
	StartTime   time.Time
	EndTime     time.Time
	WattHours   int64
	CostInCents *int64
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
func ParseReader(readCloser io.ReadCloser) ([]ElectricUsage, error) {
	defer func() {
		if err := readCloser.Close(); err != nil {
			fmt.Println("Error: failed to close response body")
		}
	}()
	reader := readCloser.(io.Reader)
	if debug {
		_, _ = fmt.Fprintln(os.Stderr, "\nDEBUG: Response from poll endpoint:")
		reader = io.TeeReader(readCloser, os.Stderr)
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
	var usageSeries, costSeries []SmartHubPoint
	for _, data := range datas {
		switch data.Type {
		case "USAGE":
			usageSeries = data.Series[0].Data
		case "COST":
			costSeries = data.Series[0].Data
		}
	}
	// this is dumb, but the SmartHub api returns "unix timestamps"
	// that are based on EST (which is incorrect), at least as of 2/29/2024.
	// Example: For Midnight, Jan 1, 1970, EST, this api would return "0"
	//          However, the correct value (UTC) would be "18000".
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, err
	}
	_, offset := time.Now().In(zone).Zone()
	period := time.UnixMilli(usageSeries[1].UnixMillis).Sub(time.UnixMilli(usageSeries[0].UnixMillis))
	records := make([]ElectricUsage, len(usageSeries))
	for i := range usageSeries {
		usage := usageSeries[i]
		// see note above about "unix timestamps"
		start := time.UnixMilli(usage.UnixMillis).Add(time.Second * time.Duration(-offset))
		records[i].StartTime = start
		records[i].EndTime = start.Add(period)
		records[i].WattHours = int64(usage.Value * 1000)
	}
	// note: cost is not returned by all SmartHub implementations. So this is a no-op sometimes.
	for i := range costSeries {
		cost := int64(costSeries[i].Value * 100)
		records[i].CostInCents = &cost
	}
	return records, nil
}
