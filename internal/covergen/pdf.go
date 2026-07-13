package covergen

import (
	"context"
	"net/url"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func renderHTMLToPDF(ctx context.Context, htmlContent string) ([]byte, error) {
	ctx, cancel := chromedp.NewContext(ctx)
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
