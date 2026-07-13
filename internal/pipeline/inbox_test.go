package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/notify"
)

func TestPanicStop(t *testing.T) {
	ctx := context.Background()
	bot := notify.NewTelegramBot()
	
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<html><body>
			<div class="inbox-row" data-id="12345">
				<a href="/my/inbox/12345/">Jane Doe</a>
				<div class="message-text">Hello</div>
			</div>
			</body></html>
		`))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	dc := client.NewDjinniClient(cfg)
	dc.Client.BaseURL = srv.URL
	
	panicStop := &atomic.Bool{}
	panicStop.Store(true) // Trigger early exit
	
	logs, err := ProcessInbox(ctx, bot, panicStop, nil, cfg, llm.Engine("ollama"), ".", dc, true)
	if err != nil {
		t.Fatalf("ProcessInbox returned error: %v", err)
	}
	
	foundPanicLog := false
	for _, l := range logs {
		if l == "🛑 PanicStop triggered, breaking Inbox loop." {
			foundPanicLog = true
			break
		}
	}
	
	if !foundPanicLog {
		t.Errorf("expected panic stop log but did not find it in logs: %v", logs)
	}
}
