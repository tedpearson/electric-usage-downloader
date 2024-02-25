package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

func DownloadCsv(config *Config, startDate string, endDate string) (string, error) {
	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil {
		return "", err
	}

	allocatorFlags := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", config.Headless))
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), allocatorFlags...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan string, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*browser.EventDownloadProgress); ok {
			if ev.TotalBytes != 0 {
				//fmt.Printf("State: %s, completed: %.2f, total: %.2f\n", ev.State.String(), ev.ReceivedBytes, ev.TotalBytes)
				if ev.State == browser.DownloadProgressStateCompleted {
					done <- ev.GUID
					close(done)
					log.Println("Download completed")
				}
			}
		}
	})

	err = chromedp.Run(ctx,
		chromedp.Navigate(config.LoginUrl),
		chromedp.SetValue("(//input)[1]", config.Username, chromedp.NodeVisible),
		chromedp.SetValue("(//input)[2]", config.Password),
		chromedp.Sleep(time.Second),
		chromedp.Click(`//button[contains(.,"Sign In")]`),
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(config.DownloadDir).
			WithEventsEnabled(true),
	)
	if err != nil {
		return "", err
	}
	// possible modal dialog that needs to be dismissed
	// if it doesn't show up, ignore error finding it
	modalCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = chromedp.Run(modalCtx,
		chromedp.Click(`//div[contains(@class, "modal-body")]//button[.="No"]`, chromedp.NodeVisible),
	)
	err = chromedp.Run(ctx,
		chromedp.Click(`//button[.="USAGE"]`, chromedp.NodeVisible),
		chromedp.Click(`//a[.='Usage Management']`, chromedp.NodeVisible),
		chromedp.Click(`//a[contains(text(), "Download Your Data")]`, chromedp.NodeVisible),
		chromedp.Click(`//app-usage-management-download-your-data//button[contains(., "Download")]`, chromedp.NodeVisible),
		chromedp.Sleep(time.Second),
		chromedp.Click(`//div[contains(@class, "interval")]//mat-select`),
		chromedp.Sleep(time.Second),
		chromedp.Click(`//mat-option[contains(., "HOURLY")]`, chromedp.NodeVisible),
		chromedp.SetValue(`//div[contains(@class, "start-picker")]//input`, startDate),
		chromedp.SetValue(`//div[contains(@class, "end-picker")]//input`, endDate),
		chromedp.Click(`//div[contains(@class, "file-format")]//mat-select`),
		chromedp.Click(`//mat-option[contains(., "CSV")]`, chromedp.NodeVisible),
		chromedp.Sleep(time.Second),
		chromedp.Click(`//button[.="Download"]`),
	)
	if err != nil {
		return "", err
	}
	log.Println("Waiting for Chrome...")
	select {
	case <-time.After(timeout):
		return "", errors.New("error: Timed out waiting for Chrome")
	case guid := <-done:
		return fmt.Sprintf("%s/%s", config.DownloadDir, guid), nil
	}
}
