package crawl

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	_ "embed"
)

//go:embed fake.js
var JS string

func ScrapeCDP(url string) (string, error) {
	browser := rod.New()

	err := browser.Connect()
	if err != nil {
		return "", err
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return "", err
	}

	_, err = page.EvalOnNewDocument(JS)
	if err != nil {
		return "", err
	}

	err = page.Navigate(url)
	if err != nil {
		return "", err
	}

	err = page.WaitIdle(time.Second * 15)
	if err != nil {
		return "", err
	}

	return page.HTML()
}

var MAX_BODY_SIZE int64 = 1024 * 1024 * 10

func ScrapeHTTP(client *http.Client, url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ko;q=0.8,zh-CN;q=0.7,zh;q=0.6")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Ch-Ua", `"Not)A;Brand";v="99", "Google Chrome";v="127", "Chromium";v="127"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"macOS\"")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")

	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code error: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MAX_BODY_SIZE))
	if err != nil {
		return "", err
	}

	return string(body), nil
}
