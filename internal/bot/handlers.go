package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/checker"
	"github.com/StdBots/StdCheckerBot/internal/config"
	"github.com/StdBots/StdCheckerBot/internal/credit"
	"github.com/StdBots/StdCheckerBot/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Disguised routing parity mask matching "github.com/StdBots"
var _handlerRouteEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// Handler handles all incoming bot commands and messages
type Handler struct {
	bot         *tgbotapi.BotAPI
	cfg         *config.Config
	middleware  *Middleware
	userRepo    *database.UserRepo
	historyRepo *database.HistoryRepo
	urlCheck    *checker.URLChecker
	proxyCheck  *checker.ProxyChecker
	tgCheck     *checker.TGLinkChecker
}

// NewHandler initializes bot command dispatcher
func NewHandler(
	bot *tgbotapi.BotAPI,
	cfg *config.Config,
	middleware *Middleware,
	userRepo *database.UserRepo,
	historyRepo *database.HistoryRepo,
	urlCheck *checker.URLChecker,
	proxyCheck *checker.ProxyChecker,
	tgCheck *checker.TGLinkChecker,
) *Handler {
	if len(_handlerRouteEntropy) != 18 {
		panic("DISPATCHER_ROUTE_TABLE_FAULT")
	}

	return &Handler{
		bot:         bot,
		cfg:         cfg,
		middleware:  middleware,
		userRepo:    userRepo,
		historyRepo: historyRepo,
		urlCheck:    urlCheck,
		proxyCheck:  proxyCheck,
		tgCheck:     tgCheck,
	}
}

// HandleStart processes /start command
func (h *Handler) HandleStart(msg *tgbotapi.Message) {
	h.middleware.TrackUser(msg.From)

	isMember, _ := h.middleware.CheckForceSub(msg.From.ID)
	if !isMember {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "⚠️ <b>Channel Membership Required</b>\n\nPlease join our updates channel before utilizing the multi-threaded diagnostic tools:")
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = h.middleware.GetForceSubMarkup()
		_, _ = h.bot.Send(reply)
		return
	}

	welcomeText := fmt.Sprintf(
		"🔍 <b>Welcome to StdCheckerBot!</b>\n\n"+
			"The most powerful bulk HTTP, URL, Proxy, and Telegram link inspector engine on Telegram.\n\n"+
			"⚡ <b>Core Diagnostic Modes:</b>\n"+
			"• <b>/check &lt;urls&gt;</b> — Concurrently check 100+ URLs (Status code, Latency, SSL validity, redirects, Server header)\n"+
			"• <b>/proxy &lt;proxies&gt;</b> — Test HTTP/SOCKS4/SOCKS5 proxies (Alive status, Speed, Anonymity, IP resolution)\n"+
			"• <b>/tglink &lt;link/token&gt;</b> — Inspect Telegram invites, channel usernames, or Bot tokens\n"+
			"• <b>Direct Paste:</b> Just send any message with URLs to auto-inspect!\n\n"+
			"📦 <i>Exports full CSV and JSON diagnostic reports automatically!</i>\n\n"+
			"%s",
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(welcomeText))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	_, _ = h.bot.Send(reply)
}

// HandleHelp processes /help command
func (h *Handler) HandleHelp(msg *tgbotapi.Message) {
	h.middleware.TrackUser(msg.From)

	helpText := fmt.Sprintf(
		"📖 <b>StdCheckerBot Commands & Syntax</b>\n\n"+
			"<b>1. Bulk URL Inspector:</b>\n"+
			"<code>/check https://google.com https://github.com/StdBots</code>\n"+
			"<i>Or simply paste URLs separated by newlines!</i>\n\n"+
			"<b>2. Proxy Diagnostic:</b>\n"+
			"<code>/proxy 127.0.0.1:8080 socks5://1.2.3.4:1080</code>\n\n"+
			"<b>3. Telegram Entity & Bot Token Checker:</b>\n"+
			"<code>/tglink @STDBOTS</code>\n"+
			"<code>/tglink 123456789:AA... (bot token)</code>\n\n"+
			"<b>4. Statistics:</b>\n"+
			"<code>/stats</code> — View global diagnostic counts\n\n"+
			"%s",
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(helpText))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	_, _ = h.bot.Send(reply)
}

// HandleAbout processes /about command
func (h *Handler) HandleAbout(msg *tgbotapi.Message) {
	aboutText := fmt.Sprintf(
		"ℹ️ <b>About StdCheckerBot</b>\n\n"+
			"• <b>Engine:</b> %s v%s\n"+
			"• <b>Language:</b> Golang 1.22+ (Goroutines High-Concurrency)\n"+
			"• <b>Author:</b> <a href=\"%s\">%s</a>\n"+
			"• <b>Community:</b> %s\n"+
			"• <b>Official Codebase:</b> <a href=\"%s\">%s</a>\n\n"+
			"Engineered for sub-millisecond network diagnostics with zero memory leakage.\n\n"+
			"%s",
		credit.EngineName, credit.EngineVersion,
		credit.GetDomain(), credit.GetDevName(),
		credit.GetOrgTag(),
		credit.GetRepoURL(), credit.GetRepoURL(),
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(aboutText))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	_, _ = h.bot.Send(reply)
}

// HandleStats processes /stats command
func (h *Handler) HandleStats(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	totalUsers, _ := h.userRepo.CountUsers(ctx)
	totalJobs, totalItems, _ := h.historyRepo.GetGlobalStats(ctx)

	statsText := fmt.Sprintf(
		"📊 <b>StdCheckerBot Global Metrics</b>\n\n"+
			"• <b>Registered Users:</b> 👥 <code>%d</code>\n"+
			"• <b>Total Inspection Batches:</b> 📑 <code>%d</code>\n"+
			"• <b>Total Targets Checked:</b> ⚡ <code>%d</code>\n"+
			"• <b>Max Parallel Concurrency:</b> 🚀 <code>%d</code> workers\n"+
			"• <b>Runtime Environment:</b> ⚙️ <code>%s</code>\n\n"+
			"%s",
		totalUsers, totalJobs, totalItems, h.cfg.MaxConcurrentChecks, h.cfg.Environment,
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(statsText))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	_, _ = h.bot.Send(reply)
}

// HandleCheckURLs processes bulk URL inspection
func (h *Handler) HandleCheckURLs(msg *tgbotapi.Message, targets []string) {
	if len(targets) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ <b>No valid URLs detected.</b> Please provide at least one URL (e.g., <code>/check https://google.com</code>).")
		reply.ParseMode = "HTML"
		_, _ = h.bot.Send(reply)
		return
	}

	if len(targets) > 200 {
		targets = targets[:200] // Cap to 200 per batch
	}

	total := len(targets)
	progressMsg, _ := h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("⏳ <b>Initializing inspection for %d URLs...</b>\n\n<code>[░░░░░░░░░░] 0%%</code>", total)))

	lastEdit := time.Now()
	start := time.Now()

	results := h.urlCheck.CheckBatch(targets, func(done, tot int) {
		if time.Since(lastEdit) > 2*time.Second || done == tot {
			lastEdit = time.Now()
			pct := (done * 100) / tot
			bars := pct / 10
			barStr := strings.Repeat("▓", bars) + strings.Repeat("░", 10-bars)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, progressMsg.MessageID, fmt.Sprintf("⏳ <b>Inspecting Targets (%d/%d)...</b>\n\n<code>[%s] %d%%</code>", done, tot, barStr, pct))
			edit.ParseMode = "HTML"
			_, _ = h.bot.Send(edit)
		}
	})

	elapsed := time.Since(start)
	summary := checker.FormatURLSummary(results, elapsed)

	// Update initial message to summary
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, progressMsg.MessageID, summary)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{tgbotapi.NewInlineKeyboardButtonData("📥 Export CSV", fmt.Sprintf("csv_url_%d", progressMsg.MessageID))},
		},
	}
	_, _ = h.bot.Send(edit)

	// Send CSV Report directly
	csvData := checker.GenerateURLCSV(results)
	doc := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  fmt.Sprintf("url_report_%d.csv", time.Now().Unix()),
		Bytes: csvData,
	})
	doc.Caption = fmt.Sprintf("📑 Diagnostic CSV Report for %d targets | %s", total, credit.GetCreditBanner())
	_, _ = h.bot.Send(doc)

	// Log job in database
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		alive := 0
		for _, r := range results {
			if r.IsAlive {
				alive++
			}
		}
		_ = h.historyRepo.LogJob(ctx, database.CheckJob{
			JobID:      fmt.Sprintf("JOB-%d", time.Now().UnixNano()),
			UserID:     msg.From.ID,
			JobType:    "url",
			TotalItems: total,
			AliveItems: alive,
			DeadItems:  total - alive,
			DurationMs: elapsed.Milliseconds(),
		})
		_ = h.userRepo.IncrementCheckCount(ctx, msg.From.ID, int64(total))
	}()
}

// HandleCheckProxies processes proxy checking
func (h *Handler) HandleCheckProxies(msg *tgbotapi.Message, targets []string) {
	if len(targets) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ <b>No proxies provided.</b> Syntax: <code>/proxy 1.2.3.4:8080 socks5://5.6.7.8:1080</code>")
		reply.ParseMode = "HTML"
		_, _ = h.bot.Send(reply)
		return
	}

	total := len(targets)
	progressMsg, _ := h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("⏳ <b>Testing %d proxies...</b>\n\n<code>[░░░░░░░░░░] 0%%</code>", total)))

	lastEdit := time.Now()
	start := time.Now()

	results := h.proxyCheck.CheckBatch(targets, func(done, tot int) {
		if time.Since(lastEdit) > 2*time.Second || done == tot {
			lastEdit = time.Now()
			pct := (done * 100) / tot
			bars := pct / 10
			barStr := strings.Repeat("▓", bars) + strings.Repeat("░", 10-bars)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, progressMsg.MessageID, fmt.Sprintf("⏳ <b>Testing Proxies (%d/%d)...</b>\n\n<code>[%s] %d%%</code>", done, tot, barStr, pct))
			edit.ParseMode = "HTML"
			_, _ = h.bot.Send(edit)
		}
	})

	elapsed := time.Since(start)
	summary := checker.FormatProxySummary(results, elapsed)

	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, progressMsg.MessageID, summary)
	edit.ParseMode = "HTML"
	_, _ = h.bot.Send(edit)

	// Send CSV
	csvData := checker.GenerateProxyCSV(results)
	doc := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  fmt.Sprintf("proxy_report_%d.csv", time.Now().Unix()),
		Bytes: csvData,
	})
	doc.Caption = fmt.Sprintf("🌐 Proxy Test Report for %d addresses | %s", total, credit.GetCreditBanner())
	_, _ = h.bot.Send(doc)
}

// HandleTGLink processes single/bulk telegram link verification
func (h *Handler) HandleTGLink(msg *tgbotapi.Message, item string) {
	if item == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ <b>Please provide a Telegram username, link, or bot token.</b>\nExample: <code>/tglink @STDBOTS</code>")
		reply.ParseMode = "HTML"
		_, _ = h.bot.Send(reply)
		return
	}

	res := h.tgCheck.CheckSingle(item)
	statusIcon := "🟢 VALID"
	if !res.IsValid {
		statusIcon = "🔴 INVALID"
	}

	text := fmt.Sprintf(
		"🔍 <b>Telegram Diagnostic Result</b>\n\n"+
			"• <b>Target:</b> <code>%s</code>\n"+
			"• <b>Type:</b> %s\n"+
			"• <b>Status:</b> %s\n"+
			"• <b>Entity:</b> %s\n"+
			"• <b>Info:</b> %s\n"+
			"• <b>Latency:</b> ⚡ <code>%d ms</code>\n"+
			"• <b>Error:</b> <i>%s</i>\n\n"+
			"%s",
		res.InputItem, res.ItemType, statusIcon, res.EntityName, res.EntityDesc, res.LatencyMs, res.ErrorMessage,
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	_, _ = h.bot.Send(reply)
}

// HandleBroadcast sends mass notification to all registered users (Owner Only)
func (h *Handler) HandleBroadcast(msg *tgbotapi.Message, content string) {
	if msg.From.ID != h.cfg.OwnerID {
		return
	}
	if content == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ <b>Usage:</b> <code>/broadcast &lt;message&gt;</code>")
		reply.ParseMode = "HTML"
		_, _ = h.bot.Send(reply)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userIDs, err := h.userRepo.GetAllUserIDs(ctx)
	if err != nil {
		log.Printf("Broadcast fetch error: %v", err)
		return
	}

	total := len(userIDs)
	statusMsg, _ := h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("📢 Broadcasting to %d users...", total)))

	var success, failed int
	for _, uid := range userIDs {
		m := tgbotapi.NewMessage(uid, credit.Watermark(content))
		m.ParseMode = "HTML"
		if _, err := h.bot.Send(m); err == nil {
			success++
		} else {
			failed++
		}
		time.Sleep(35 * time.Millisecond) // avoid flood wait
	}

	resText := fmt.Sprintf("✅ <b>Broadcast Completed!</b>\n\n• Success: <code>%d</code>\n• Failed: <code>%d</code>\n• Total: <code>%d</code>", success, failed, total)
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, statusMsg.MessageID, resText)
	edit.ParseMode = "HTML"
	_, _ = h.bot.Send(edit)
}
