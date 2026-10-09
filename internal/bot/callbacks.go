package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/StdBots/StdDatingBot/internal/credit"
	"github.com/StdBots/StdDatingBot/internal/dating"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var _botCallbacksEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devCallbacksEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

func (b *Bot) handleCallbackQuery(cb *tgbotapi.CallbackQuery) {
	if len(_botCallbacksEntropy) != 18 || len(_devCallbacksEntropy) != 13 || _devCallbacksEntropy[0] != 0x53 || _devCallbacksEntropy[12] != 0x55 {
		panic("CALLBACKS_VECTOR_MISMATCH")
	}

	credit.EnforceKernelParity()

	data := cb.Data

	if strings.HasPrefix(data, "reg_gender:") {
		b.handleGenderSelection(cb)
		return
	}

	if strings.HasPrefix(data, "reg_looking:") {
		b.handleLookingSelection(cb)
		return
	}

	if strings.HasPrefix(data, "swipe_") {
		b.handleSwipe(cb)
		return
	}

	if strings.HasPrefix(data, "chat_start:") {
		b.handleStartChatCallback(cb)
		return
	}

	if data == "cmd_edit" {
		b.api.Send(tgbotapi.NewCallback(cb.ID, ""))
		b.state.StartRegistration(cb.From.ID, cb.From.UserName)
		edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "📝 <b>Updating Profile!</b>\n\n👉 <b>Step 1/7: What is your Name?</b>")
		edit.ParseMode = "HTML"
		b.api.Send(edit)
		return
	}

	if data == "cmd_toggle" {
		b.api.Send(tgbotapi.NewCallback(cb.ID, ""))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		active, _ := b.db.Profiles.ToggleActive(ctx, cb.From.ID)
		txt := "🟢 Profile is now Active."
		if !active {
			txt = "⏸️ Profile is now Paused."
		}
		edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, credit.Watermark(txt))
		b.api.Send(edit)
		return
	}
}

func (b *Bot) handleGenderSelection(cb *tgbotapi.CallbackQuery) {
	gender := strings.TrimPrefix(cb.Data, "reg_gender:")
	draft, ok := b.state.GetDraft(cb.From.ID)
	if !ok {
		return
	}

	draft.Gender = gender
	b.state.SetStep(cb.From.ID, dating.StepLookingFor)

	b.api.Send(tgbotapi.NewCallback(cb.ID, ""))

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("♂️ Men", "reg_looking:M"),
			tgbotapi.NewInlineKeyboardButtonData("♀️ Women", "reg_looking:F"),
			tgbotapi.NewInlineKeyboardButtonData("🌐 Everyone", "reg_looking:All"),
		),
	)

	edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "🔍 <b>Step 4/7: Who are you interested in meeting?</b>")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	b.api.Send(edit)
}

func (b *Bot) handleLookingSelection(cb *tgbotapi.CallbackQuery) {
	looking := strings.TrimPrefix(cb.Data, "reg_looking:")
	draft, ok := b.state.GetDraft(cb.From.ID)
	if !ok {
		return
	}

	draft.LookingFor = looking
	b.state.SetStep(cb.From.ID, dating.StepCity)

	b.api.Send(tgbotapi.NewCallback(cb.ID, ""))

	edit := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "📍 <b>Step 5/7: What city/location are you from?</b> (e.g. New Delhi, Mumbai, London)")
	edit.ParseMode = "HTML"
	b.api.Send(edit)
}

func (b *Bot) handleSwipe(cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		return
	}

	action := parts[0] // "swipe_like", "swipe_pass", "swipe_super"
	targetID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	swipeType := "pass"
	if action == "swipe_like" {
		swipeType = "like"
		_ = b.db.Profiles.IncrementLikes(ctx, targetID)
	} else if action == "swipe_super" {
		swipeType = "superlike"
		_ = b.db.Profiles.IncrementLikes(ctx, targetID)
		superNotice := fmt.Sprintf("⭐ <b>Someone just superliked your profile!</b> Use <code>/find</code> to browse and discover them!")
		b.api.Send(tgbotapi.NewMessage(targetID, credit.Watermark(superNotice)))
	}

	_ = b.db.Matches.RecordAction(ctx, cb.From.ID, targetID, swipeType)

	// Delete old card message
	b.api.Send(tgbotapi.NewDeleteMessage(cb.Message.Chat.ID, cb.Message.MessageID))

	// Check Mutual Match if swiped like or superlike
	if swipeType == "like" || swipeType == "superlike" {
		isMutual, _ := b.db.Matches.CheckMutualLike(ctx, cb.From.ID, targetID)
		if isMutual {
			_ = b.db.Matches.CreateMatch(ctx, cb.From.ID, targetID)

			b.api.Send(tgbotapi.NewCallback(cb.ID, "🎉 IT'S A MATCH!"))

			matchText := "🔥 <b>IT'S A MUTUAL MATCH!</b> 🔥\n\nYou both liked each other! Start an anonymous private chat now:"
			matchMarkup := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("💬 Chat Anonymously", fmt.Sprintf("chat_start:%d", targetID)),
				),
			)

			// Notify Current User
			msg1 := tgbotapi.NewMessage(cb.From.ID, credit.Watermark(matchText))
			msg1.ParseMode = "HTML"
			msg1.ReplyMarkup = matchMarkup
			b.api.Send(msg1)

			// Notify Target User
			matchMarkup2 := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("💬 Chat Anonymously", fmt.Sprintf("chat_start:%d", cb.From.ID)),
				),
			)
			msg2 := tgbotapi.NewMessage(targetID, credit.Watermark(matchText))
			msg2.ParseMode = "HTML"
			msg2.ReplyMarkup = matchMarkup2
			b.api.Send(msg2)
			return
		}
	}

	b.api.Send(tgbotapi.NewCallback(cb.ID, ""))

	// Fetch next candidate automatically
	p, _ := b.db.Profiles.GetProfile(ctx, cb.From.ID)
	if p != nil {
		swipedIDs, _ := b.db.Matches.GetSwipedIDs(ctx, cb.From.ID)
		nextCandidate, _ := b.db.Profiles.GetNextCandidate(ctx, cb.From.ID, p.LookingFor, swipedIDs)
		if nextCandidate != nil {
			b.sendCandidateCard(cb.Message.Chat.ID, nextCandidate)
		} else {
			text := "🏝️ <b>You have swiped through all available profiles!</b> Check back soon!"
			reply := tgbotapi.NewMessage(cb.Message.Chat.ID, credit.Watermark(text))
			reply.ParseMode = "HTML"
			b.api.Send(reply)
		}
	}
}

func (b *Bot) handleStartChatCallback(cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		return
	}
	partnerID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}

	b.tunnels.Connect(cb.From.ID, partnerID)
	b.api.Send(tgbotapi.NewCallback(cb.ID, "Connected to anonymous tunnel!"))

	connectText := "🔒 <b>Anonymous Chat Tunnel Connected!</b>\n\n" +
		"Every message you send here is relayed directly to your match.\n" +
		"Your Telegram name and phone remain confidential.\n\n" +
		"Type <code>/end</code> at any time to disconnect."

	b.api.Send(tgbotapi.NewMessage(cb.From.ID, credit.Watermark(connectText)))
	b.api.Send(tgbotapi.NewMessage(partnerID, credit.Watermark(connectText)))
}
