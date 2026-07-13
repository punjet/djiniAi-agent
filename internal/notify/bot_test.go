package notify

import (
	"os"
	"testing"
	"time"
)

func TestTelegramBot(t *testing.T) {
	// Temporarily set env vars
	oldToken := os.Getenv("TG_BOT_TOKEN")
	defer os.Setenv("TG_BOT_TOKEN", oldToken)

	os.Setenv("TG_BOT_TOKEN", "mock-token")

	bot := NewTelegramBot()
	
	cmds := map[string]func(*TGMessage){
		"/test": func(msg *TGMessage) {},
	}
	bot.Commands(cmds)

	bot.SetLastSummary("hello summary")
	if bot.GetLastSummary() != "hello summary" {
		t.Errorf("expected summary to be 'hello summary'")
	}

	bot.Start()
	time.Sleep(100 * time.Millisecond) // let goroutine run a bit
	bot.Stop()
	
	// Double stop should be safe
	bot.Stop()
	
	// PanicStop should be safe
	bot.PanicStop()

	// start and stop without token
	os.Setenv("TG_BOT_TOKEN", "")
	bot2 := NewTelegramBot()
	bot2.Start()
	bot2.Stop()
}

func TestTelegramBotCommands(t *testing.T) {
	os.Setenv("TG_CHAT_ID", "12345")
	defer os.Setenv("TG_CHAT_ID", "")

	var sentMessages []string
	SendMessageFunc = func(text string) error {
		sentMessages = append(sentMessages, text)
		return nil
	}
	defer func() { SendMessageFunc = SendTelegramMessage }()

	bot := NewTelegramBot()
	bot.SetLastSummary("All good.")
	bot.Start() // make it running

	// Helper to simulate incoming messages
	simulateCmd := func(cmd string, chatID int64) {
		msg := &TGMessage{
			Chat: TGChat{ID: chatID},
			Text: cmd,
		}
		bot.mu.RLock()
		cmdFunc, ok := bot.commands[cmd]
		bot.mu.RUnlock()
		if ok {
			cmdFunc(msg)
		}
	}

	simulateCmd("/start", 12345)
	simulateCmd("/status", 12345)
	simulateCmd("/report", 12345)
	simulateCmd("/stop", 12345) // will stop the bot
	simulateCmd("/panic", 12345) // will panic stop

	if len(sentMessages) != 5 {
		t.Errorf("expected 5 messages to be sent, got %d", len(sentMessages))
	}
}
