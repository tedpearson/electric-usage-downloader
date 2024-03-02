package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"
)

type ElectricUsage struct {
	StartTime   time.Time
	EndTime     time.Time
	WattHours   int64
	CostInCents int64
}

type Response struct {
	Status string                 `json:"status"`
	Data   map[string][]NovecData `json:"data"`
}

type NovecData struct {
	Type   string        `json:"type"`
	Series []NovecSeries `json:"series"`
}

type NovecSeries struct {
	Data []NovecPoint `json:"data"`
}

type NovecPoint struct {
	UnixMillis int64   `json:"x"`
	Value      float64 `json:"y"`
}

type RetryableError struct {
	Msg string
}

func NewRetryableError(msg string) *RetryableError {
	return &RetryableError{Msg: msg}
}

func (t *RetryableError) Error() string {
	return t.Msg
}

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
