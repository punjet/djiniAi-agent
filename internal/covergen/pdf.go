package covergen

import (
	"context"
	"net/url"
	

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/carlos7ags/folio/document"
)

func renderHTMLToPDFChromedp(ctx context.Context, htmlContent string) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("headless", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	htmlURI := "data:text/html," + url.PathEscape(htmlContent)

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(htmlURI),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			buf = data
			return err
		}),
	)

	return buf, err
}

func renderHTMLToPDFFolio(ctx context.Context, htmlContent string) ([]byte, error) {
	doc := document.NewDocument(document.PageSizeA4)
	doc.AddHTML(htmlContent, nil)
	buf := new(bytes.Buffer)
	_, err := doc.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderHTMLToPDF(ctx context.Context, htmlContent string) ([]byte, error) {
	return renderHTMLToPDFFolio(ctx, htmlContent)
}