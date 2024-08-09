package main

import (
	"os"

	"github.com/lemon-mint/infofluss/internal/crawl"
	"github.com/lemon-mint/infofluss/internal/htmldistill"
)

func main() {
	url := "https://ai.google.dev/gemini-api/docs/json-mode?lang=python"
	html, err := crawl.ScrapeCDP(url)
	if err != nil {
		panic(err)
	}

	os.WriteFile("cmd/crawltest/index.html", []byte(html), 0644)

	distilled, err := htmldistill.Clean(html)
	if err != nil {
		panic(err)
	}

	os.WriteFile("cmd/crawltest/index.distilled.html", []byte(distilled), 0644)

	text, err := htmldistill.ExtractText(distilled)
	if err != nil {
		panic(err)
	}

	os.WriteFile("cmd/crawltest/index.text.txt", []byte(text), 0644)
}
