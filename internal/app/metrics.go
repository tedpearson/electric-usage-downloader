package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type MetricLine struct {
	Timestamps []int64
}

func WriteMetrics(records []*ElectricUsage, config InfluxDB, existingPoints map[int64]struct{}) error {
	client := influxdb2.NewClient(config.Host, config.User+":"+config.Password)
	writeApi := client.WriteAPIBlocking("", config.Database)
	for _, record := range records {
		minutes := record.EndTime.Sub(record.StartTime).Minutes()
		points := make([]*write.Point, 0, int(minutes))
		multiplier := 60 / minutes
		for t := record.StartTime; record.EndTime.After(t); t = t.Add(time.Minute) {
			if _, ok := existingPoints[t.UnixMilli()]; ok {
				continue
			}
			point := influxdb2.NewPointWithMeasurement("electric").
				SetTime(t).
				AddField("watts", float64(record.WattHours)*multiplier).
				AddField("cost", float64(record.CostInCents)/minutes)
			points = append(points, point)
		}
		err := writeApi.WritePoint(context.Background(), points...)
		if err != nil {
			return err
		}
	}
	return nil
}

func QueryPreviousMetrics(startTime time.Time, endTime time.Time, config InfluxDB) (map[int64]struct{}, error) {
	client := &http.Client{}
	v := url.Values{
		"match[]": {"electric_usage"},
		"start":   {startTime.Format(`2006-01-02T15:04:05Z07:00`)},
		"end":     {endTime.Format("2006-01-02T15:04:05Z07:00")},
	}
	req, err := http.NewRequest("POST", config.Host+"/api/v1/export", strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.User, config.Password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	existing := make(map[int64]struct{})
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var line MetricLine
		if err := decoder.Decode(&line); err != nil {
			log.Println(fmt.Errorf("Bad line: %w\n", err))
		}
		for _, ts := range line.Timestamps {
			existing[ts] = struct{}{}
		}
	}
	return existing, nil
}
