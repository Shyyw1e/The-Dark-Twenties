package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	userclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/user"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const serviceName = "telegram-bot-polling"

type Service struct {
	bot         *tgbotapi.BotAPI
	users       *userclient.Client
	log         logger.Logger
	pollTimeout int

	cancel context.CancelFunc
	done   chan struct{}
}

func NewService(bot *tgbotapi.BotAPI, users *userclient.Client, cfg config.TelegramConfig, log logger.Logger) *Service {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if cfg.PollTimeoutSeconds <= 0 {
		cfg.PollTimeoutSeconds = 30
	}

	return &Service{
		bot:         bot,
		users:       users,
		log:         log,
		pollTimeout: cfg.PollTimeoutSeconds,
		done:        make(chan struct{}),
	}
}

func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.bot == nil {
		return errors.New("telegram bot api is nil")
	}
	if s.users == nil {
		return errors.New("user-service client is nil")
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = s.pollTimeout

	updates := s.bot.GetUpdatesChan(updateConfig)

	go s.run(runCtx, updates)

	s.log.Info("telegram bot polling started", "username", s.bot.Self.UserName)
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.bot != nil {
		s.bot.StopReceivingUpdates()
	}

	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) run(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	defer close(s.done)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("telegram bot polling stopped")
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			s.handleUpdate(ctx, update)
		}
	}
}

func (s *Service) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	if text != "/start" {
		return
	}

	user, err := s.users.GetOrCreateTelegramUser(ctx, userclient.TelegramUserInput{
		TelegramID:   update.Message.From.ID,
		Username:     update.Message.From.UserName,
		FirstName:    update.Message.From.FirstName,
		LastName:     update.Message.From.LastName,
		LanguageCode: update.Message.From.LanguageCode,
	})
	if err != nil {
		s.log.Error("failed to get or create telegram user", "telegram_id", update.Message.From.ID, "error", err)
		s.sendText(update.Message.Chat.ID, "Не получилось зарегистрировать аккаунт. Попробуйте еще раз чуть позже.")
		return
	}

	s.log.Info(
		"telegram user registered",
		"user_id", user.GetId(),
		"telegram_id", user.GetTelegramId(),
	)

	s.sendText(update.Message.Chat.ID, fmt.Sprintf("Привет! Аккаунт готов. Ваш статус: %s.", user.GetStatus()))
}

func (s *Service) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := s.bot.Send(msg); err != nil {
		s.log.Error("failed to send telegram message", "chat_id", chatID, "error", err)
	}
}
