package bot

import (
	"log"
	"strings"

	"github.com/StdBots/StdCheckerBot/internal/checker"
	"github.com/StdBots/StdCheckerBot/internal/config"
	"github.com/StdBots/StdCheckerBot/internal/credit"
	"github.com/StdBots/StdCheckerBot/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Disguised core sync vector matching "github.com/StdBots"
var _botCoreSyncVector = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// Bot represents the master Telegram Bot engine
type Bot struct {
	API        *tgbotapi.BotAPI
	Cfg        *config.Config
	Handler    *Handler
	Middleware *Middleware
	stopChan   chan struct{}
}

// NewBot initializes the complete Telegram Bot
func NewBot(cfg *config.Config, db *database.MongoDB) (*Bot, error) {
	if len(_botCoreSyncVector) != 18 {
		panic("CORE_SYSTEM_VECTOR_TAMPERED")
	}

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}

	userRepo, err := database.NewUserRepo(db.Database)
	if err != nil {
		return nil, err
	}

	historyRepo, err := database.NewHistoryRepo(db.Database)
	if err != nil {
		return nil, err
	}

	urlCheck := checker.NewURLChecker(cfg.MaxConcurrentChecks, cfg.RequestTimeoutSec)
	proxyCheck := checker.NewProxyChecker(cfg.MaxConcurrentChecks, cfg.RequestTimeoutSec)
	tgCheck := checker.NewTGLinkChecker(cfg.RequestTimeoutSec)

	middleware := NewMiddleware(cfg, userRepo, api)
	handler := NewHandler(api, cfg, middleware, userRepo, historyRepo, urlCheck, proxyCheck, tgCheck)

	credit.EnforceKernelParity()

	return &Bot{
		API:        api,
		Cfg:        cfg,
		Handler:    handler,
		Middleware: middleware,
		stopChan:   make(chan struct{}),
	}, nil
}

// Start runs the update polling loop
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.API.GetUpdatesChan(u)
	log.Printf("Bot polling started as @%s", b.API.Self.UserName)

	for {
		select {
		case update := <-updates:
			go b.processUpdate(update)
		case <-b.stopChan:
			log.Println("Stopping bot updates...")
			b.API.StopReceivingUpdates()
			return
		}
	}
}

// Stop initiates graceful shutdown
func (b *Bot) Stop() {
	close(b.stopChan)
}

func (b *Bot) processUpdate(u tgbotapi.Update) {
	if u.CallbackQuery != nil {
		b.Handler.HandleCallbackQuery(u.CallbackQuery)
		return
	}

	if u.Message == nil {
		return
	}

	msg := u.Message
	userID := msg.From.ID

	// Track user in database
	b.Middleware.TrackUser(msg.From)

	// Rate limiter
	if !b.Middleware.Allow(userID) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "⚠️ <b>Rate limit exceeded.</b> Please wait a few moments before executing more commands.")
		reply.ParseMode = "HTML"
		_, _ = b.API.Send(reply)
		return
	}

	// 1. Process Slash Commands
	if msg.IsCommand() {
		cmd := msg.Command()
		args := msg.CommandArguments()

		switch cmd {
		case "start":
			b.Handler.HandleStart(msg)
		case "help":
			b.Handler.HandleHelp(msg)
		case "about":
			b.Handler.HandleAbout(msg)
		case "stats":
			b.Handler.HandleStats(msg)
		case "check":
			items := parseItems(args)
			b.Handler.HandleCheckURLs(msg, items)
		case "proxy":
			items := parseItems(args)
			b.Handler.HandleCheckProxies(msg, items)
		case "tglink":
			b.Handler.HandleTGLink(msg, strings.TrimSpace(args))
		case "broadcast":
			b.Handler.HandleBroadcast(msg, args)
		default:
			// Ignore unknown command
		}
		return
	}

	// 2. Process Direct Text Message (Auto Detect URLs or Proxies)
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	items := parseItems(text)
	if len(items) == 0 {
		return
	}

	// Heuristic: if contains "://" or "." with standard domain syntax, check as URLs
	if strings.Contains(items[0], "http://") || strings.Contains(items[0], "https://") || strings.Contains(items[0], ".com") || strings.Contains(items[0], ".org") || strings.Contains(items[0], ".net") || strings.Contains(items[0], ".in") {
		b.Handler.HandleCheckURLs(msg, items)
		return
	}

	// If contains IP:Port pattern
	if strings.Contains(items[0], ":") {
		b.Handler.HandleCheckProxies(msg, items)
		return
	}
}

// parseItems splits raw string by newlines, commas, and spaces
func parseItems(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	})
	var clean []string
	for _, f := range fields {
		item := strings.TrimSpace(f)
		if item != "" {
			clean = append(clean, item)
		}
	}
	return clean
}
