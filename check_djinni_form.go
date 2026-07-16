package main

import (
	"fmt"
	"os"
	"strings"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"github.com/joho/godotenv"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	_ = godotenv.Overload(".env")
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	dc := client.NewDjinniClient(cfg)
	
	resp, err := dc.Client.R().Get("https://djinni.co/jobs/837081-crm-ai-automation-engineer-kommo-n8n-apis/")
	if err != nil {
		panic(err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resp.String()))
	if err != nil {
		panic(err)
	}
	
	form := doc.Find("form#job-apply-form")
	if form.Length() == 0 {
		fmt.Println("No apply form found! Maybe already applied or unauthorized?")
		os.WriteFile("debug_job.html", []byte(resp.String()), 0644)
		return
	}
	
	fmt.Println("Form inputs:")
	form.Find("input, textarea, select").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		typ, _ := s.Attr("type")
		val, _ := s.Attr("value")
		fmt.Printf("- %s (type: %s, value: %s)\n", name, typ, val)
	})
}
