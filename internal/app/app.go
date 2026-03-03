package app

import (
	"context"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/aliskhannn/gopher-kicker-bot/internal/config"
	"github.com/aliskhannn/gopher-kicker-bot/internal/infra/redis"
	"github.com/aliskhannn/gopher-kicker-bot/internal/service"
)

type App struct {
	bot     *tgbotapi.BotAPI
	service *service.Bot
	logger  *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		return nil, err
	}

	storage, err := redis.New(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	botService := service.New(bot, storage, cfg)

	return &App{
		bot:     bot,
		service: botService,
		logger:  logger,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.logger.Info("starting bot", zap.String("version", "1.0"))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := a.bot.GetUpdatesChan(u)
	
	go a.timeoutChecker(ctx)

	for update := range updates {
		if err := a.handleUpdate(ctx, update); err != nil {
			a.logger.Error("handle update failed", zap.Error(err))
		}
	}

	return nil
}

func (a *App) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.ChatJoinRequest != nil {
		return a.service.HandleJoinRequest(ctx, update.ChatJoinRequest)
	}

	if update.Message != nil && update.Message.From != nil {
		return a.service.HandleUserMessage(ctx, update.Message)
	}

	return nil
}

func (a *App) timeoutChecker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.service.CheckTimeouts(ctx); err != nil {
				a.logger.Error("timeout check failed", zap.Error(err))
			}
		}
	}
}

func (a *App) Stop() {
	a.bot.StopReceivingUpdates()
}
