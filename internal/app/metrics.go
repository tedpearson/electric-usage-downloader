package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

//type Metric struct {
//	Name string `json:"__name__"`
//	Db   string
//}

type MetricLine struct {
	//Metric     Metric
	//Values     []float64
	Timestamps []int64
}

func WriteMetrics(records []*ElectricUsage, config InfluxDB) {
	client := influxdb2.NewClient(config.Host, config.User+":"+config.Password)
	writeApi := client.WriteAPIBlocking("", config.Database)
	points := make([]*write.Point, 0, 15*2*len(records))
	for _, record := range records {
		divisor := record.EndTime.Sub(record.StartTime).Minutes()
		for t := record.StartTime; record.EndTime.After(t); t = t.Add(time.Minute) {
			watts := influxdb2.NewPointWithMeasurement("electric").
				SetTime(t).
				AddField("usage", float64(record.WattHours)/divisor)
			cost := influxdb2.NewPointWithMeasurement("electric").
				SetTime(t).
				AddField("cost", float64(record.CostInCents)/divisor)
			points = append(points, watts, cost)
		}
	}

	err := writeApi.WritePoint(context.Background(), points...)
	if err != nil {
		log.Fatal(err)
	}
	// query VM for metrics that already exist in the range we're trying to insert?
	// if that's too much work, then just maintain last-inserted-time and don't insert newer
	log.Println(points)
}

// the goal here is to not double write any metrics
// therefore, we should simply filter inserted points by any points that
// already exist.
// the algorithm for that is to run this query first, making a map[time]struct{}
// and discarding any point in WriteMetrics that exists in the map.

func QueryPreviousMetrics(startTime time.Time, endTime time.Time, config InfluxDB) map[int64]struct{} {
	client := &http.Client{}
	v := url.Values{
		"match[]": {"sensor_temperature"},
		"start":   {startTime.Format("2006-01-02T15:04:05+07:00")},
		"end":     {endTime.Format("2006-01-02T15:04:05+07:00")},
	}
	req, err := http.NewRequest("POST", config.Host+"/api/v1/export", strings.NewReader(v.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.User, config.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
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
			log.Printf("Bad line: %w\n", err)
		}
		for _, ts := range line.Timestamps {
			existing[ts] = struct{}{}
		}
	}
	return existing
}
