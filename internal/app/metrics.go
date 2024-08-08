package app

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// WriteMetrics writes ElectricUsage data to victoriametrics.
// This method writes a point every minute instead of following the time span of ElectricUsage.
func WriteMetrics(records []ElectricUsage, config InfluxConfig) error {
	opts := influxdb2.DefaultOptions()
	if config.Insecure {
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}
	client := influxdb2.NewClientWithOptions(config.Host, config.AuthToken, opts)
	writeApi := client.WriteAPIBlocking(config.Org, config.Database)
	for _, record := range records {
		minutes := record.EndTime.Sub(record.StartTime).Minutes()
		points := make([]*write.Point, 0, int(minutes))
		multiplier := 60 / minutes
		for t := record.StartTime; record.EndTime.After(t); t = t.Add(time.Minute) {
			point := influxdb2.NewPointWithMeasurement("electric").
				SetTime(t).
				AddField("watts", float64(record.WattHours)*multiplier)
			if record.CostInCents != nil {
				point.AddField("cost", float64(*record.CostInCents)/minutes)
			}
			if record.MeterName != nil {
				point.AddTag("name", *record.MeterName)
			}
			points = append(points, point)
		}
		err := writeApi.WritePoint(context.Background(), points...)
		if err != nil {
			return err
		}
	}
	return nil
}
