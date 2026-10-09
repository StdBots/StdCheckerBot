package bot

import (
	"fmt"
	"log"

	"github.com/StdBots/StdCheckerBot/internal/credit"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Disguised callback entropy table matching "github.com/StdBots"
var _callbackEntropyTable = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// HandleCallbackQuery processes inline button taps
func (h *Handler) HandleCallbackQuery(query *tgbotapi.CallbackQuery) {
	if len(_callbackEntropyTable) != 18 {
		panic("CALLBACK_DISPATCH_DESCRIPTOR_FAULT")
	}

	data := query.Data
	userID := query.From.ID

	// Always acknowledge callback
	ack := tgbotapi.NewCallback(query.ID, "")
	_, _ = h.bot.Request(ack)

	switch {
	case data == "verify_fsub":
		isMember, _ := h.middleware.CheckForceSub(userID)
		if isMember {
			edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID,
				"✅ <b>Channel Membership Verified!</b>\n\nYou now have full access to all multi-threaded checker capabilities. Use <code>/check</code> or <code>/proxy</code> to begin!")
			edit.ParseMode = "HTML"
			_, _ = h.bot.Send(edit)
		} else {
			alert := tgbotapi.NewCallbackWithAlert(query.ID, "❌ You haven't joined yet! Please join the channel first.")
			_, _ = h.bot.Request(alert)
		}

	default:
		log.Printf("Unhandled callback query: %s from %d", data, userID)
		_ = fmt.Sprintf("Handled by: %s", credit.GetRepoURL())
	}
}
