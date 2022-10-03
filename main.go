package main

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

const loginUrl string = "https://novec.smarthub.coop/Login.html"

var (
	startDate = "09/29/2022"
	endDate   = "09/30/2022"
)

type Config struct {
	Username string
	Password string
}

func main() {

	// read config
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()

	//ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	//defer cancel()

	done := make(chan string, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*browser.EventDownloadProgress); ok {
			if ev.TotalBytes != 0 {
				fmt.Printf("State: %s, completed: %.2f, total: %.2f\n", ev.State.String(), ev.ReceivedBytes, ev.TotalBytes)
				if ev.State == browser.DownloadProgressStateCompleted {
					done <- ev.GUID
					close(done)
				}
			}
		}
	})

	chromedp.Run(ctx,
		chromedp.Navigate(loginUrl),
		//chromedp.WaitVisible("#LoginUsernameTextBox"),
		chromedp.SetValue("#LoginUsernameTextBox", config.Username, chromedp.NodeVisible),
		chromedp.SetValue("#LoginPasswordTextBox", config.Password),
		chromedp.Click("#LoginSubmitButton"),
		//chromedp.WaitVisible(`//a[. = "Log Out"]`),
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath("/tmp").
			WithEventsEnabled(true),
		chromedp.Click("#MyUsageDropDown > a", chromedp.NodeVisible),
		chromedp.Click(`//div[.="Usage Explorer"]`, chromedp.NodeVisible),
		chromedp.Click(`//img[@alt='Usage Management']`, chromedp.NodeVisible),
		chromedp.Click(`(//input[@name="timeFrameRadio"])[3]`, chromedp.NodeVisible),
		chromedp.SetValue(`(//input[contains(@class, "form-control-readonly")])[1]`, startDate),
		chromedp.SetValue(`(//input[contains(@class, "form-control-readonly")])[2]`, endDate),
		chromedp.Click(`(//input[@name="fileFormatRadio"])[2]`),
		chromedp.Click(`//button[.="Download Usage Data"]`),
	)
	guid := <-done
	fmt.Printf("hello %s", guid)
}
