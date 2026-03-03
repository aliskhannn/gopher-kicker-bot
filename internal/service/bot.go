package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/aliskhannn/gopher-kicker-bot/internal/config"
	"github.com/aliskhannn/gopher-kicker-bot/internal/infra/redis"
	"github.com/aliskhannn/gopher-kicker-bot/internal/models"
)

type Storage interface {
	SetPending(ctx context.Context, state *models.UserState) error
	GetPending(ctx context.Context, userID int64) (*models.UserState, bool, error)
	GetAllPending(ctx context.Context) ([]*models.UserState, error)
	DeletePending(ctx context.Context, userID int64) error
}

type Bot struct {
	bot     *tgbotapi.BotAPI
	storage Storage
	cfg     *config.Config
}

func New(bot *tgbotapi.BotAPI, storage *redis.Storage, cfg *config.Config) *Bot {
	return &Bot{
		bot:     bot,
		storage: storage,
		cfg:     cfg,
	}
}

func (b *Bot) HandleJoinRequest(ctx context.Context, join *tgbotapi.ChatJoinRequest) error {
	x, y := rand.Intn(50)+1, rand.Intn(50)+1
	task := fmt.Sprintf("%d + %d = ?", x, y)
	correctAnswer := x + y

	now := time.Now()
	state := &models.UserState{
		UserID:    join.From.ID,
		ChatID:    b.cfg.Telegram.ChatID,
		State:     "waiting_answer",
		Task:      task,
		Answer:    correctAnswer,
		Username:  join.From.UserName,
		SentAt:    now,
		WarnAt:    now.Add(time.Duration(b.cfg.Bot.WarnAfterSec) * time.Second),
		TimeoutAt: now.Add(time.Duration(b.cfg.Bot.TimeoutSec) * time.Second),
	}

	msgText := fmt.Sprintf(`Ассаляму алейкум! Добро пожаловать!

Реши простую задачу для подтверждения заявки на вступление:
<b>%s</b>

У тебя есть <b>%d минут</b> на ответ.
Напиши только число!`, state.Task, b.cfg.Bot.TimeoutSec/60)

	msg := tgbotapi.NewMessage(join.Chat.ID, msgText)
	msg.ParseMode = tgbotapi.ModeHTML

	sentMsg, err := b.bot.Send(msg)
	if err != nil {
		decline := tgbotapi.DeclineChatJoinRequest{
			ChatConfig: tgbotapi.ChatConfig{
				ChatID: b.cfg.Telegram.ChatID,
			},
			UserID: join.From.ID,
		}
		_, _ = b.bot.Request(decline)
		return fmt.Errorf("send PM: %w", err)
	}

	state.QuestionID = sentMsg.MessageID

	return b.storage.SetPending(ctx, state)
}

func (b *Bot) HandleUserMessage(ctx context.Context, msg *tgbotapi.Message) error {
	userID := msg.From.ID

	state, exists, err := b.storage.GetPending(ctx, userID)
	if err != nil || !exists {
		return nil
	}

	now := time.Now()

	if now.After(state.TimeoutAt) {
		return b.declineUser(ctx, state)
	}

	if now.After(state.WarnAt) && state.WarnSentAt.IsZero() {
		return b.warnUser(ctx, state)
	}

	userAnswer, err := strconv.Atoi(strings.TrimSpace(msg.Text))
	if err != nil || userAnswer != state.Answer {
		return b.remindWrongAnswer(userID, state.Task)
	}

	return b.approveUser(ctx, state)
}

func (b *Bot) approveUser(ctx context.Context, state *models.UserState) error {
	approve := tgbotapi.ApproveChatJoinRequestConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: b.cfg.Telegram.ChatID,
		},
		UserID: state.UserID,
	}

	if _, err := b.bot.Request(approve); err != nil {
		return fmt.Errorf("approve join request: %w", err)
	}

	approveMsg := tgbotapi.NewMessage(state.UserID, "✅ Отлично! Математика на уровне! Заявка одобрена, добро пожаловать в чат! 🐹✨")
	_, _ = b.bot.Send(approveMsg)

	return b.storage.DeletePending(ctx, state.UserID)
}

func (b *Bot) declineUser(ctx context.Context, state *models.UserState) error {
	decline := tgbotapi.DeclineChatJoinRequest{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: b.cfg.Telegram.ChatID,
		},
		UserID: state.UserID,
	}

	if _, err := b.bot.Request(decline); err != nil {
		return fmt.Errorf("decline join request: %w", err)
	}

	kickMsg := tgbotapi.NewMessage(state.UserID, "⏰ Время вышло (или попытки исчерпаны). Твоя заявка отклонена. Подай заявку заново и реши задачу! 🧮")
	_, _ = b.bot.Send(kickMsg)

	return b.storage.DeletePending(ctx, state.UserID)
}

func (b *Bot) warnUser(ctx context.Context, state *models.UserState) error {
	warnMsg := tgbotapi.NewMessage(state.UserID, "⏳ Осталось мало времени! Быстрее решай задачу:\n<b>"+state.Task+"</b>\n\nИначе заявка будет отклонена! ⏰")
	warnMsg.ParseMode = tgbotapi.ModeHTML

	if _, err := b.bot.Send(warnMsg); err != nil {
		return err
	}

	state.WarnSentAt = time.Now()
	return b.storage.SetPending(ctx, state)
}

func (b *Bot) remindWrongAnswer(userID int64, task string) error {
	msg := tgbotapi.NewMessage(userID, fmt.Sprintf("❌ Неправильно! Задача: <b>%s</b>\nПопробуй еще раз!", task))
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := b.bot.Send(msg)
	return err
}

func (b *Bot) CheckTimeouts(ctx context.Context) error {
	states, err := b.storage.GetAllPending(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, state := range states {
		if now.After(state.TimeoutAt) {
			if err := b.declineUser(ctx, state); err != nil {
				fmt.Printf("declining user %d: %v\n", state.UserID, err)
			}
			continue
		}

		if now.After(state.WarnAt) && state.WarnSentAt.IsZero() {
			if err := b.warnUser(ctx, state); err != nil {
				fmt.Printf("warning user %d: %v\n", state.UserID, err)
			}
		}
	}
	return nil
}
