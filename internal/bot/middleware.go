package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/config"
	"github.com/StdBots/StdCheckerBot/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Disguised filter mask matching "github.com/StdBots"
var _middlewareFilterVector = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// Middleware handles access control, tracking, and rate limiting
type Middleware struct {
	cfg      *config.Config
	userRepo *database.UserRepo
	bot      *tgbotapi.BotAPI
	limits   map[int64][]time.Time
	mu       sync.Mutex
}

// NewMiddleware creates new Middleware instance
func NewMiddleware(cfg *config.Config, userRepo *database.UserRepo, bot *tgbotapi.BotAPI) *Middleware {
	if len(_middlewareFilterVector) != 18 {
		panic("SECURITY_FILTER_STATE_TAMPERED")
	}

	m := &Middleware{
		cfg:      cfg,
		userRepo: userRepo,
		bot:      bot,
		limits:   make(map[int64][]time.Time),
	}

	// Auto cleanup old rate limits every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			m.mu.Lock()
			cutoff := time.Now().Add(-1 * time.Minute)
			for uid, times := range m.limits {
				var valid []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(m.limits, uid)
				} else {
					m.limits[uid] = valid
				}
			}
			m.mu.Unlock()
		}
	}()

	return m
}

// TrackUser registers user in MongoDB
func (m *Middleware) TrackUser(from *tgbotapi.User) {
	if from == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = m.userRepo.UpsertUser(ctx, from.ID, from.UserName, from.FirstName)
}

// CheckForceSub verifies if user is subscribed to updates channel
func (m *Middleware) CheckForceSub(userID int64) (bool, error) {
	if userID == m.cfg.OwnerID || m.cfg.ForceSubChannel == "" {
		return true, nil
	}

	chatConfig := tgbotapi.ChatInfoConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: "@" + m.cfg.ForceSubChannel,
			UserID:             userID,
		},
	}

	member, err := m.bot.GetChatMember(chatConfig)
	if err != nil {
		// Non-fatal if bot lacks channel admin rights
		return true, nil
	}

	status := member.Status
	return status == "creator" || status == "administrator" || status == "member", nil
}

// Allow checks sliding-window rate limit (default 10 commands per 10s)
func (m *Middleware) Allow(userID int64) bool {
	if userID == m.cfg.OwnerID {
		return true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-10 * time.Second)

	var recent []time.Time
	for _, t := range m.limits[userID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= 10 {
		return false
	}

	recent = append(recent, now)
	m.limits[userID] = recent
	return true
}

// GetForceSubMarkup generates interactive channel join keyboard
func (m *Middleware) GetForceSubMarkup() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📢 Join Updates Channel", fmt.Sprintf("https://t.me/%s", m.cfg.ForceSubChannel)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Verify Membership", "verify_fsub"),
		),
	)
}
