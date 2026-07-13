package notify

import (
	"log"
	"os"
	"fmt"
	"strings"
	"sync"
	"time"
)

type TelegramBot struct {
	commands    map[string]func(*TGMessage)
	lastSummary string
	mu          sync.RWMutex
	stopChan    chan struct{}
	running     bool
	offset      int64
	statusMsgID int64
	UpdateChan  chan TGUpdate
}

// SendMessageFunc allows mocking the Telegram message sender in tests.
var SendMessageFunc = SendTelegramMessage

func NewTelegramBot() *TelegramBot {
	bot := &TelegramBot{
		commands: make(map[string]func(*TGMessage)),
		UpdateChan: make(chan TGUpdate, 100),
	}
	bot.SetupDefaultCommands()
	return bot
}

func (b *TelegramBot) verifyChat(m *TGMessage) bool {
	expected := os.Getenv("TG_CHAT_ID")
	if expected != "" && fmt.Sprintf("%d", m.Chat.ID) != expected {
		return false
	}
	return true
}

func (b *TelegramBot) buildStatusText() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.running {
		return "Pipeline Status: Running"
	}
	return "Pipeline Status: Stopped"
}

func (b *TelegramBot) SetupDefaultCommands() {
	b.Commands(map[string]func(*TGMessage){
		"/start": func(m *TGMessage) {
			if !b.verifyChat(m) { return }
			SendMessageFunc("Hello! I am your Djinni Bot. Commands: /start, /status, /stop, /panic, /report")
		},
		"/status": func(m *TGMessage) {
			if !b.verifyChat(m) { return }
			SendMessageFunc(b.buildStatusText())
		},
		"/stop": func(m *TGMessage) {
			if !b.verifyChat(m) { return }
			SendMessageFunc("Stopping bot...")
			b.Stop()
		},
		"/panic": func(m *TGMessage) {
			if !b.verifyChat(m) { return }
			SendMessageFunc("PANIC! Halting pipeline...")
			b.PanicStop()
		},
		"/report": func(m *TGMessage) {
			if !b.verifyChat(m) { return }
			summary := b.GetLastSummary()
			if summary == "" {
				summary = "No recent run summary available."
			}
			SendMessageFunc("Last Run Summary:\n" + summary)
		},
	})
}

func (b *TelegramBot) Commands(cmds map[string]func(*TGMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commands = cmds
}

func (b *TelegramBot) SetLastSummary(summary string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastSummary = summary
}

func (b *TelegramBot) GetLastSummary() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastSummary
}

func (b *TelegramBot) Start() {
	if os.Getenv("TG_BOT_TOKEN") == "" {
		return
	}

	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.stopChan = make(chan struct{})
	b.mu.Unlock()

	// Drain stale updates
	updates, err := b.GetUpdates()
	if err == nil && len(updates) > 0 {
		b.offset = updates[len(updates)-1].UpdateID + 1
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-b.stopChan:
				return
			case <-ticker.C:
				updates, err := b.GetUpdates()
				if err != nil {
					log.Printf("Telegram GetUpdates error: %v", err)
					continue
				}
				for _, update := range updates {
					b.offset = update.UpdateID + 1
					if update.Message != nil && strings.HasPrefix(update.Message.Text, "/") {
						cmdStr := strings.Split(update.Message.Text, " ")[0]
						b.mu.RLock()
						cmdFunc, ok := b.commands[cmdStr]
						b.mu.RUnlock()
						if ok {
							cmdFunc(update.Message)
						}
					} else {
						select {
						case b.UpdateChan <- update:
						default:
							// drop if channel is full
						}
					}
				}
			}
		}
	}()
}

func (b *TelegramBot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		close(b.stopChan)
		b.running = false
	}
}

func (b *TelegramBot) PanicStop() {
	b.Stop()
}

func (b *TelegramBot) GetUpdates() ([]TGUpdate, error) {
	return GetUpdates(b.offset)
}

func (b *TelegramBot) StartStatusBoard() {
	if os.Getenv("TG_BOT_TOKEN") == "" || os.Getenv("TG_CHAT_ID") == "" {
		return
	}

	msgID, err := SendTelegramMessageID(b.buildStatusText())
	if err != nil {
		log.Printf("Failed to send status board message: %v", err)
		return
	}

	_ = PinChatMessage(msgID)

	b.mu.Lock()
	b.statusMsgID = msgID
	b.mu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-b.stopChan:
				// Update one last time to reflect stopped status
				_ = EditMessageText(msgID, b.buildStatusText())
				return
			case <-ticker.C:
				_ = EditMessageText(msgID, b.buildStatusText())
			}
		}
	}()
}
