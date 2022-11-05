package app

import (
	"context"
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
		chromedp.SetValue("#LoginUsernameTextBox", config.Username, chromedp.NodeVisible),
		chromedp.SetValue("#LoginPasswordTextBox", config.Password),
		chromedp.Click("#LoginSubmitButton"),
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
		chromedp.Click("#ViewUsageLink", chromedp.NodeVisible),
		chromedp.Click(`//div[.="Usage Explorer"]`, chromedp.NodeVisible),
		chromedp.Click(`//img[@alt='Usage Management']`, chromedp.NodeVisible),
		chromedp.Sleep(time.Second),
		chromedp.Click(`(//input[@name="timeFrameRadio"])[3]`, chromedp.NodeVisible),
		chromedp.SetValue(`(//input[contains(@class, "form-control-readonly")])[1]`, startDate),
		chromedp.SetValue(`(//input[contains(@class, "form-control-readonly")])[2]`, endDate),
		chromedp.Click(`(//input[@name="fileFormatRadio"])[2]`),
		chromedp.Click(`//button[.="Download Usage Data"]`),
	)
	if err != nil {
		return "", err
	}
	log.Println("Waiting for Chrome...")
	guid := <-done
	return fmt.Sprintf("%s/%s", config.DownloadDir, guid), nil
}
