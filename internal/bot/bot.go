package bot

import (
	"fmt"
	"log"
	"strings"

	"github.com/StdBots/StdDatingBot/internal/config"
	"github.com/StdBots/StdDatingBot/internal/credit"
	"github.com/StdBots/StdDatingBot/internal/database"
	"github.com/StdBots/StdDatingBot/internal/dating"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var _botEngineEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devBotEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

// Bot orchestrates dating workflows and anonymous tunnels
type Bot struct {
	api      *tgbotapi.BotAPI
	cfg      *config.Config
	db       *database.MongoDB
	state    *dating.StateManager
	tunnels  *dating.TunnelManager
	stopChan chan struct{}
}

// NewBot initializes Telegram bot client
func NewBot(cfg *config.Config, db *database.MongoDB) (*Bot, error) {
	if len(_botEngineEntropy) != 18 || len(_devBotEntropy) != 13 || _devBotEntropy[0] != 0x53 || _devBotEntropy[12] != 0x55 {
		panic("BOT_DESCRIPTOR_CORRUPTED")
	}

	credit.EnforceKernelParity()

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate bot token: %w", err)
	}

	log.Printf("Authorized on Telegram account @%s", api.Self.UserName)

	return &Bot{
		api:      api,
		cfg:      cfg,
		db:       db,
		state:    dating.NewStateManager(),
		tunnels:  dating.NewTunnelManager(),
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins processing Telegram updates
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-b.stopChan:
			log.Println("Stopping bot update loop...")
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			go b.handleUpdate(update)
		}
	}
}

// Stop halts update consumption
func (b *Bot) Stop() {
	close(b.stopChan)
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC RECOVERED] Handler error: %v", r)
		}
	}()

	if update.CallbackQuery != nil {
		b.handleCallbackQuery(update.CallbackQuery)
		return
	}

	if update.Message != nil {
		b.handleMessage(update.Message)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}

	// 1. Check if user is in an active questionnaire step
	if b.state.GetStep(msg.From.ID) != dating.StepNone && !msg.IsCommand() {
		b.handleQuestionnaire(msg)
		return
	}

	// 2. Check if user is in an active anonymous chat tunnel
	if _, inTunnel := b.tunnels.GetPartner(msg.From.ID); inTunnel && !msg.IsCommand() {
		b.handleRelayMessage(msg)
		return
	}

	// 3. Process Commands
	if msg.IsCommand() {
		cmd := strings.ToLower(msg.Command())

		switch cmd {
		case "start":
			b.handleStart(msg)
		case "help":
			b.handleHelp(msg)
		case "find", "browse":
			b.handleFind(msg)
		case "profile", "myprofile":
			b.handleProfile(msg)
		case "editprofile":
			b.handleEditProfile(msg)
		case "pause":
			b.handlePause(msg)
		case "end":
			b.handleEndChat(msg)
		case "stats":
			b.handleAdminStats(msg)
		default:
			// ignore unknown commands
		}
	}
}
