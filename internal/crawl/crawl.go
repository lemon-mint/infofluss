package crawl

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/lemon-mint/coord/llm"
	"github.com/rs/zerolog/log"

	_ "embed"
)

// Be sure to close pools/instances when you're done with them.
var pool pdfium.Pool

func init() {
	var err error
	// Init the PDFium library and return the instance to open documents.
	// You can tweak these configs to your need. Be aware that workers can use quite some memory.
	pool, err = webassembly.Init(webassembly.Config{
		MinIdle:  1, // Makes sure that at least x workers are always available
		MaxIdle:  1, // Makes sure that at most x workers are ever available
		MaxTotal: 1, // Maximum amount of workers in total, allows the amount of workers to grow when needed, items between total max and idle max are automatically cleaned up, while idle workers are kept alive so they can be used directly.
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init pdfium pool")
	}
}

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

	page.WaitIdle(time.Second * 10)

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

func ScrapeCDPImages(url string) ([]llm.InlineData, error) {
	browser := rod.New()

	err := browser.Connect()
	if err != nil {
		return nil, err
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}

	_, err = page.EvalOnNewDocument(JS)
	if err != nil {
		return nil, err
	}

	err = page.Navigate(url)
	if err != nil {
		return nil, err
	}

	err = page.WaitLoad()
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Second * 1)

	pdf, err := page.PDF(&proto.PagePrintToPDF{})
	if err != nil {
		return nil, err
	}

	defer pdf.Close()
	pdfBytes, err := io.ReadAll(pdf)
	if err != nil {
		return nil, err
	}

	images, err := convertToImages(pdfBytes)
	if err != nil {
		return nil, err
	}

	var inlineData []llm.InlineData
	for _, image := range images {
		inlineData = append(inlineData, llm.InlineData{
			Data:     image,
			MIMEType: "image/jpeg",
		})
	}

	return inlineData, nil
}

func convertToImages(pdf []byte) ([][]byte, error) {
	instance, err := pool.GetInstance(time.Second * 30)
	if err != nil {
		return nil, err
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{
		File: &pdf,
	})
	if err != nil {
		return nil, err
	}

	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return nil, err
	}

	var pages [][]byte
	var b bytes.Buffer

	for i := range pageCount.PageCount {
		pageRender, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
			DPI: 300,
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		img := pageRender.Result.Image
		b.Reset()
		err = jpeg.Encode(&b, img, nil)
		if err != nil {
			return nil, err
		}

		pages = append(pages, append([]byte(nil), b.Bytes()...))
	}

	instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	return pages, nil
}
