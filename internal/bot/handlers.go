package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/StdBots/StdDatingBot/internal/credit"
	"github.com/StdBots/StdDatingBot/internal/database"
	"github.com/StdBots/StdDatingBot/internal/dating"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var _botHandlersEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devHandlersEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	if len(_botHandlersEntropy) != 18 || len(_devHandlersEntropy) != 13 || _devHandlersEntropy[0] != 0x53 || _devHandlersEntropy[12] != 0x55 {
		panic("HANDLERS_VECTOR_MISMATCH")
	}

	p, err := b.GetUserProfile(msg.From)
	if err != nil || p == nil {
		// New user: initiate registration questionnaire
		b.state.StartRegistration(msg.From.ID, msg.From.UserName)
		welcome := fmt.Sprintf(
			"💘 <b>Welcome to %s!</b>\n\n"+
				"Find connections, chat anonymously, and meet amazing people right inside Telegram.\n\n"+
				"📝 Let's set up your dating profile in just a few quick steps!\n\n"+
				"👉 <b>Step 1/7: What is your Name?</b>",
			credit.BotTag,
		)
		reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(welcome))
		reply.ParseMode = "HTML"
		b.api.Send(reply)
		return
	}

	// Returning user
	text := fmt.Sprintf(
		"👋 <b>Welcome back, %s!</b>\n\n"+
			"Ready to meet someone new?\n\n"+
			"• 🔍 <code>/find</code> — Start browsing potential matches\n"+
			"• 👤 <code>/profile</code> — View your profile card\n"+
			"• ⚙️ <code>/editprofile</code> — Update your details\n"+
			"• ⏸️ <code>/pause</code> — Pause/Resume profile visibility\n"+
			"• 🛑 <code>/end</code> — Terminate active anonymous chat\n\n"+
			"%s",
		p.Name,
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	b.api.Send(reply)
}

func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	text := fmt.Sprintf(
		"📖 <b>%s — Commands & User Guide</b>\n\n"+
			"<b>Matchmaking:</b>\n"+
			"• <code>/find</code> — Browse profiles (Like ❤️, Pass 👎, Superlike ⭐)\n"+
			"• <code>/profile</code> — Check your current profile appearance\n"+
			"• <code>/editprofile</code> — Restart questionnaire\n"+
			"• <code>/pause</code> — Hide/Unhide your card from discovery\n\n"+
			"<b>Anonymous Chat Tunnel:</b>\n"+
			"When you match with someone, you can chat directly 1-on-1.\n"+
			"Your Telegram username and phone remain 100%% anonymous.\n"+
			"• <code>/end</code> — Close current active chat tunnel\n\n"+
			"%s",
		credit.BotTag,
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = credit.GetInlineButtons()
	b.api.Send(reply)
}

func (b *Bot) handleFind(msg *tgbotapi.Message) {
	p, err := b.GetUserProfile(msg.From)
	if err != nil || p == nil {
		b.sendError(msg.Chat.ID, "Please create your profile first with <code>/start</code>.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	swipedIDs, _ := b.db.Matches.GetSwipedIDs(ctx, msg.From.ID)
	candidate, err := b.db.Profiles.GetNextCandidate(ctx, msg.From.ID, p.LookingFor, swipedIDs)
	if err != nil {
		b.sendError(msg.Chat.ID, "Database error finding candidates.")
		return
	}

	if candidate == nil {
		text := "🏝️ <b>You have swiped through all available profiles in your filter!</b>\n\nCheck back soon or invite friends to join!"
		reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
		reply.ParseMode = "HTML"
		b.api.Send(reply)
		return
	}

	b.sendCandidateCard(msg.Chat.ID, candidate)
}

func (b *Bot) sendCandidateCard(chatID int64, c *database.Profile) {
	genderSym := "♂️"
	if c.Gender == "F" {
		genderSym = "♀️"
	} else if c.Gender == "Other" {
		genderSym = "⚧️"
	}

	caption := fmt.Sprintf(
		"✨ <b>%s, %d %s</b>\n"+
			"📍 <i>%s</i>\n\n"+
			"💬 %s\n\n"+
			"❤️ Total Likes: <b>%d</b>",
		c.Name,
		c.Age,
		genderSym,
		c.City,
		c.Bio,
		c.TotalLikes,
	)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👎 Pass", fmt.Sprintf("swipe_pass:%d", c.ID)),
			tgbotapi.NewInlineKeyboardButtonData("⭐ Super", fmt.Sprintf("swipe_super:%d", c.ID)),
			tgbotapi.NewInlineKeyboardButtonData("❤️ Like", fmt.Sprintf("swipe_like:%d", c.ID)),
		),
	)

	if c.PhotoFileID != "" {
		photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(c.PhotoFileID))
		photoMsg.Caption = credit.Watermark(caption)
		photoMsg.ParseMode = "HTML"
		photoMsg.ReplyMarkup = markup
		b.api.Send(photoMsg)
	} else {
		textMsg := tgbotapi.NewMessage(chatID, credit.Watermark(caption))
		textMsg.ParseMode = "HTML"
		textMsg.ReplyMarkup = markup
		b.api.Send(textMsg)
	}
}

func (b *Bot) handleProfile(msg *tgbotapi.Message) {
	p, err := b.GetUserProfile(msg.From)
	if err != nil || p == nil {
		b.sendError(msg.Chat.ID, "No profile found. Use <code>/start</code> to create one.")
		return
	}

	genderMap := map[string]string{"M": "Male ♂️", "F": "Female ♀️", "Other": "Other ⚧️"}
	lookMap := map[string]string{"M": "Men ♂️", "F": "Women ♀️", "All": "Everyone 🌐"}

	status := "🟢 Active"
	if !p.IsActive {
		status = "⏸️ Paused (Hidden)"
	}

	caption := fmt.Sprintf(
		"👤 <b>Your Dating Profile Card</b>\n\n"+
			"• <b>Name:</b> %s\n"+
			"• <b>Age:</b> %d\n"+
			"• <b>Gender:</b> %s\n"+
			"• <b>Looking For:</b> %s\n"+
			"• <b>City:</b> %s\n"+
			"• <b>Bio:</b> %s\n"+
			"• <b>Status:</b> %s\n"+
			"• <b>Likes Received:</b> %d ❤️\n\n"+
			"%s",
		p.Name,
		p.Age,
		genderMap[p.Gender],
		lookMap[p.LookingFor],
		p.City,
		p.Bio,
		status,
		p.TotalLikes,
		credit.GetCreditBanner(),
	)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Edit Profile", "cmd_edit"),
			tgbotapi.NewInlineKeyboardButtonData("⏸️ Pause/Resume", "cmd_toggle"),
		),
	)

	if p.PhotoFileID != "" {
		photoMsg := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileID(p.PhotoFileID))
		photoMsg.Caption = credit.Watermark(caption)
		photoMsg.ParseMode = "HTML"
		photoMsg.ReplyMarkup = markup
		b.api.Send(photoMsg)
	} else {
		textMsg := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(caption))
		textMsg.ParseMode = "HTML"
		textMsg.ReplyMarkup = markup
		b.api.Send(textMsg)
	}
}

func (b *Bot) handleEditProfile(msg *tgbotapi.Message) {
	b.state.StartRegistration(msg.From.ID, msg.From.UserName)
	text := "📝 <b>Updating Profile!</b>\n\n👉 <b>Step 1/7: What is your Name?</b>"
	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
	reply.ParseMode = "HTML"
	b.api.Send(reply)
}

func (b *Bot) handlePause(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	active, err := b.db.Profiles.ToggleActive(ctx, msg.From.ID)
	if err != nil {
		b.sendError(msg.Chat.ID, "Profile error.")
		return
	}

	stateText := "🟢 Your profile is now <b>Active</b> and discoverable."
	if !active {
		stateText = "⏸️ Your profile is now <b>Paused</b> and hidden from others."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(stateText))
	reply.ParseMode = "HTML"
	b.api.Send(reply)
}

func (b *Bot) handleEndChat(msg *tgbotapi.Message) {
	partnerID, found := b.tunnels.Disconnect(msg.From.ID)
	if !found {
		b.sendError(msg.Chat.ID, "You are not currently in an active anonymous chat tunnel.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = b.db.Matches.EndMatch(ctx, msg.From.ID)

	b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark("🛑 <b>Chat session terminated.</b> You are no longer connected.")))
	b.api.Send(tgbotapi.NewMessage(partnerID, credit.Watermark("🛑 <b>Your partner has ended the chat.</b> Use <code>/find</code> to browse new matches!")))
}

func (b *Bot) handleQuestionnaire(msg *tgbotapi.Message) {
	step := b.state.GetStep(msg.From.ID)
	draft, ok := b.state.GetDraft(msg.From.ID)
	if !ok || step == dating.StepNone {
		return
	}

	switch step {
	case dating.StepName:
		name := strings.TrimSpace(msg.Text)
		if len(name) < 2 || len(name) > 30 {
			b.sendError(msg.Chat.ID, "Please enter a valid name (2-30 characters).")
			return
		}
		draft.Name = name
		b.state.SetStep(msg.From.ID, dating.StepAge)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🎂 <b>Step 2/7: How old are you?</b> (Must be 18+)")
		reply.ParseMode = "HTML"
		b.api.Send(reply)

	case dating.StepAge:
		age, err := strconv.Atoi(strings.TrimSpace(msg.Text))
		if err != nil || age < b.cfg.MinAge || age > b.cfg.MaxAge {
			b.sendError(msg.Chat.ID, fmt.Sprintf("Please enter a valid age between %d and %d.", b.cfg.MinAge, b.cfg.MaxAge))
			return
		}
		draft.Age = age
		b.state.SetStep(msg.From.ID, dating.StepGender)

		markup := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("♂️ Male", "reg_gender:M"),
				tgbotapi.NewInlineKeyboardButtonData("♀️ Female", "reg_gender:F"),
				tgbotapi.NewInlineKeyboardButtonData("⚧️ Other", "reg_gender:Other"),
			),
		)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "⚧️ <b>Step 3/7: What is your gender?</b>")
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = markup
		b.api.Send(reply)

	case dating.StepCity:
		city := strings.TrimSpace(msg.Text)
		if len(city) < 2 || len(city) > 40 {
			b.sendError(msg.Chat.ID, "Please enter a valid city name.")
			return
		}
		draft.City = city
		b.state.SetStep(msg.From.ID, dating.StepBio)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "💬 <b>Step 6/7: Write a short bio about yourself!</b> (Hobbies, interests, what you're looking for)")
		reply.ParseMode = "HTML"
		b.api.Send(reply)

	case dating.StepBio:
		bio := strings.TrimSpace(msg.Text)
		if len(bio) < 5 || len(bio) > 300 {
			b.sendError(msg.Chat.ID, "Bio should be between 5 and 300 characters.")
			return
		}
		draft.Bio = bio
		b.state.SetStep(msg.From.ID, dating.StepPhoto)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "📸 <b>Step 7/7: Send a profile photo!</b> (Or send /skip to proceed without one)")
		reply.ParseMode = "HTML"
		b.api.Send(reply)

	case dating.StepPhoto:
		if msg.Text == "/skip" {
			draft.PhotoFileID = ""
		} else if len(msg.Photo) > 0 {
			draft.PhotoFileID = msg.Photo[len(msg.Photo)-1].FileID
		} else {
			b.sendError(msg.Chat.ID, "Please upload a photo or type /skip.")
			return
		}

		// Save profile to MongoDB
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := b.db.Profiles.SaveProfile(ctx, draft); err != nil {
			b.sendError(msg.Chat.ID, "Failed to save profile.")
			return
		}

		b.state.Clear(msg.From.ID)

		successText := fmt.Sprintf(
			"🎉 <b>Congratulations, %s! Your profile is complete!</b>\n\n"+
				"Type <code>/find</code> to start meeting people right now!\n\n"+
				"%s",
			draft.Name,
			credit.GetCreditBanner(),
		)
		reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(successText))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = credit.GetInlineButtons()
		b.api.Send(reply)
	}
}

func (b *Bot) handleRelayMessage(msg *tgbotapi.Message) {
	partnerID, active := b.tunnels.GetPartner(msg.From.ID)
	if !active {
		return
	}

	// Relay message anonymously to partner
	if msg.Text != "" {
		forwardText := fmt.Sprintf("💬 <b>Partner:</b> %s", msg.Text)
		b.api.Send(tgbotapi.NewMessage(partnerID, credit.Watermark(forwardText)))
	} else if len(msg.Photo) > 0 {
		photo := tgbotapi.NewPhoto(partnerID, tgbotapi.FileID(msg.Photo[len(msg.Photo)-1].FileID))
		photo.Caption = credit.Watermark("📷 <b>Partner sent a photo</b>")
		b.api.Send(photo)
	} else if msg.Voice != nil {
		voice := tgbotapi.NewVoice(partnerID, tgbotapi.FileID(msg.Voice.FileID))
		b.api.Send(voice)
	} else if msg.Sticker != nil {
		sticker := tgbotapi.NewSticker(partnerID, tgbotapi.FileID(msg.Sticker.FileID))
		b.api.Send(sticker)
	}
}

func (b *Bot) handleAdminStats(msg *tgbotapi.Message) {
	if !b.cfg.IsAdmin(msg.From.ID) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profileCount, _ := b.db.Profiles.Count(ctx)
	matchCount, _ := b.db.Matches.CountMatches(ctx)

	text := fmt.Sprintf(
		"📊 <b>%s — Ecosystem Analytics</b>\n\n"+
			"👥 Total Dating Profiles: <b>%d</b>\n"+
			"💘 Total Matches Made: <b>%d</b>\n"+
			"⚡ Engine Version: <b>%s</b>\n"+
			"🛡️ Parity Status: <b>VERIFIED</b>\n\n"+
			"%s",
		credit.BotTag,
		profileCount,
		matchCount,
		credit.EngineVersion,
		credit.GetCreditBanner(),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, credit.Watermark(text))
	reply.ParseMode = "HTML"
	b.api.Send(reply)
}

func (b *Bot) sendError(chatID int64, errText string) {
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ %s", errText))
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}
