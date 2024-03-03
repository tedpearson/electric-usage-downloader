package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"
)

// ElectricUsage contains usage and cost information for a defined period of time.
type ElectricUsage struct {
	StartTime   time.Time
	EndTime     time.Time
	WattHours   int64
	CostInCents int64
}

// Response holds the parsed response from the Novec poll api.
// If Status is "PENDING", this means we need to make the same request again
// as data is still being prepared.
type Response struct {
	Status string                 `json:"status"`
	Data   map[string][]NovecData `json:"data"`
}

// NovecData holds parsed response data from the Novec poll api.
// It holds the Type of data ("USAGE" or "COST"), and the Series
type NovecData struct {
	Type   string        `json:"type"`
	Series []NovecSeries `json:"series"`
}

// NovecSeries holds parsed response data from the Novec poll api.
// It holds a list of NovecPoints.
type NovecSeries struct {
	Data []NovecPoint `json:"data"`
}

// NovecPoint  holds parsed response data from the Novec poll api.
// It holds a timestamp, UnixMillis, which is actually in the America/New_York timezone instead of
// UTC as it should be.
// It also holds the Value of the point in dollars or kWh.
type NovecPoint struct {
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

// ParseReader parses the json response received in FetchData from the Novec poll api.
// It can return a normal error, a RetryableError, or parsed ElectricUsage.
func ParseReader(reader io.ReadCloser) ([]ElectricUsage, error) {
	defer func() {
		if err := reader.Close(); err != nil {
			panic(err)
		}
	}()
	resp := &Response{}
	err := json.NewDecoder(reader).Decode(resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != "COMPLETE" {
		log.Println("Data not ready, retrying...")
		return nil, NewRetryableError("data processing not complete")
	}
	fmt.Println("Data received, parsing...")
	datas, ok := resp.Data["ELECTRIC"]
	if !ok {
		return nil, errors.New("no ELECTRIC key")
	}
	var usageSeries, costSeries []NovecPoint
	for _, data := range datas {
		switch data.Type {
		case "USAGE":
			usageSeries = data.Series[0].Data
		case "COST":
			costSeries = data.Series[0].Data
		}
	}
	// this is dumb, but the novec smarthub api returns "unix timestamps"
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
		cost := costSeries[i]
		// see note above about "unix timestamps"
		start := time.UnixMilli(usage.UnixMillis).Add(time.Second * time.Duration(-offset))
		records[i].StartTime = start
		records[i].EndTime = start.Add(period)
		records[i].WattHours = int64(usage.Value * 1000)
		records[i].CostInCents = int64(cost.Value * 100)
	}
	return records, nil
}
