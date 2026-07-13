package api

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"djinni-bot-go/internal/client"
)

// GetUnreadMessages fetches unread messages from /my/inbox/?bucket=unread and parses them.
func GetUnreadMessages(dc *client.DjinniClient) ([]Dialogue, error) {
	targetURL := "https://djinni.co/my/inbox/?bucket=unread"
	if dc.Client.BaseURL != "" {
		targetURL = dc.Client.BaseURL + "/my/inbox/?bucket=unread"
	}
	resp, err := dc.Client.R().Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch inbox: %w", err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("unexpected status code fetching inbox: %d", resp.StatusCode)
	}

	htmlContent := resp.String()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var dialogues []Dialogue
	doc.Find("div.proposal, div.inbox-row, .b-list-jobs__item").Each(func(i int, s *goquery.Selection) {
		dialogID, exists := s.Attr("data-id")
		if !exists {
			// Try finding a link to /my/inbox/ID/
			s.Find("a[href^='/my/inbox/']").Each(func(j int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				parts := strings.Split(strings.Trim(href, "/"), "/")
				if len(parts) >= 3 && dialogID == "" {
					dialogID = parts[2]
				}
			})
		}
		if dialogID == "" {
			return
		}

		sender := "Unknown"
		compName := s.Find(".company_name").First()
		if compName.Length() > 0 {
			compName.Find("svg, [data-bs-toggle='tooltip']").Remove()
			sender = strings.TrimSpace(compName.Text())
			sender = strings.ReplaceAll(sender, "·", "/")
			sender = regexp.MustCompile(`\s+`).ReplaceAllString(sender, " ")
		} else {
			mobileHeader := s.Find(".header-mobile").First()
			if mobileHeader.Length() > 0 {
				sender = strings.TrimSpace(mobileHeader.Text())
				sender = regexp.MustCompile(`\s+`).ReplaceAllString(sender, " ")
			}
		}

		message := ""
		msgNode := s.Find(".message-text-inner").First()
		if msgNode.Length() > 0 {
			message = strings.TrimSpace(msgNode.Text())
			message = regexp.MustCompile(`\s+`).ReplaceAllString(message, " ")
		}

		dialogues = append(dialogues, Dialogue{
			ID:      dialogID,
			Sender:  sender,
			Message: message,
		})
	})

	return dialogues, nil
}

// ReplyToMessage posts a reply to /my/inbox/{dialogID}/ as form data.
func ReplyToMessage(dc *client.DjinniClient, dialogID string, text string) (string, error) {
	if dialogID == "" {
		return "", errors.New("dialogID cannot be empty")
	}

	url := fmt.Sprintf("https://djinni.co/my/inbox/%s/", dialogID)
	if dc.Client.BaseURL != "" {
		url = fmt.Sprintf("%s/my/inbox/%s/", dc.Client.BaseURL, dialogID)
	}

	resp, err := dc.Client.R().
		SetFormData(map[string]string{
			"message":               text,
			"template_name":         "",
			"csrfmiddlewaretoken":   dc.Config.CSRFToken,
		}).
		SetHeader("Referer", url).
		SetHeader("X-CSRFToken", dc.Config.CSRFToken).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("failed to send reply: %w", err)
	}
	if !resp.IsSuccess() {
		return "", fmt.Errorf("unexpected status code sending reply: %d", resp.StatusCode)
	}

	return resp.String(), nil
}

// GetThreadMessages fetches the full conversation thread for a given dialogID.
func GetThreadMessages(dc *client.DjinniClient, dialogID string) ([]ThreadMessage, error) {
	if dialogID == "" {
		return nil, errors.New("dialogID cannot be empty")
	}

	url := fmt.Sprintf("https://djinni.co/my/inbox/%s/", dialogID)
	if dc.Client.BaseURL != "" {
		url = fmt.Sprintf("%s/my/inbox/%s/", dc.Client.BaseURL, dialogID)
	}

	resp, err := dc.Client.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thread: %w", err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("unexpected status code fetching thread: %d", resp.StatusCode)
	}

	return parseThreadMessages(strings.NewReader(resp.String()))
}

// parseThreadMessages extracts ThreadMessage from the thread page HTML.
func parseThreadMessages(r io.Reader) ([]ThreadMessage, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var messages []ThreadMessage
	doc.Find(".b-message").Each(func(i int, s *goquery.Selection) {
		role := "candidate"
		if s.HasClass("b-message--recruiter") {
			role = "recruiter"
		}

		text := strings.TrimSpace(s.Find(".message-text-inner").Text())
		
		timeNode := s.Find("time.message-date")
		timestamp, exists := timeNode.Attr("datetime")
		if !exists {
			timestamp = strings.TrimSpace(timeNode.Text())
		}

		messages = append(messages, ThreadMessage{
			Role:      role,
			Text:      text,
			Timestamp: timestamp,
		})
	})

	if messages == nil {
		messages = []ThreadMessage{}
	}

	return messages, nil
}
