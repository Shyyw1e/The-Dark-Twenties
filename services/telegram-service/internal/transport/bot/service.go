package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	userv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/user/v1"
	configclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/config"
	subscriptionclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/subscription"
	userclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/user"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "telegram-bot-polling"

const (
	commandStart    = "/start"
	commandHelp     = "/help"
	commandMyConfig = "/my_config"
	commandReferral = "/referral"
)

const (
	callbackSubscription  = "subscription"
	callbackActivateTrial = "subscription:activate_trial"
	callbackMyConfig      = "my_config"
	callbackReferral      = "referral"
	callbackHelp          = "help"
	callbackBackToStart   = "back:start"
)

type UserClient interface {
	GetOrCreateTelegramUser(ctx context.Context, input userclient.TelegramUserInput) (*userv1.User, error)
}

type SubscriptionClient interface {
	StartTrial(ctx context.Context, userID string) (*subscriptionv1.Subscription, error)
	GetActive(ctx context.Context, userID string) (*subscriptionv1.Subscription, error)
	ActivateOrRenew(ctx context.Context, input subscriptionclient.ActivateOrRenewInput) (*subscriptionv1.Subscription, error)
}

type ConfigClient interface {
	ProvisionSubscription(ctx context.Context, input configclient.ProvisionSubscriptionInput) (*configclient.ProvisionSubscriptionOutput, error)
}

type Service struct {
	bot           *tgbotapi.BotAPI
	users         UserClient
	subscriptions SubscriptionClient
	configs       ConfigClient
	log           logger.Logger
	pollTimeout   int

	cancel context.CancelFunc
	done   chan struct{}
}

func NewService(bot *tgbotapi.BotAPI, users UserClient, subscriptions SubscriptionClient, configs ConfigClient, cfg config.TelegramConfig, log logger.Logger) *Service {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if cfg.PollTimeoutSeconds <= 0 {
		cfg.PollTimeoutSeconds = 30
	}

	return &Service{
		bot:           bot,
		users:         users,
		subscriptions: subscriptions,
		configs:       configs,
		log:           log,
		pollTimeout:   cfg.PollTimeoutSeconds,
		done:          make(chan struct{}),
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
	if s.subscriptions == nil {
		return errors.New("subscription-service client is nil")
	}
	if s.configs == nil {
		return errors.New("config-service client is nil")
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
	if update.CallbackQuery != nil {
		s.handleCallback(ctx, update.CallbackQuery)
		return
	}
	if update.Message == nil || update.Message.From == nil {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	switch text {
	case commandStart:
		s.sendStartMenu(update.Message.Chat.ID)
	case commandHelp:
		s.sendHelp(update.Message.Chat.ID)
	case commandMyConfig:
		s.sendMyConfig(ctx, update.Message.Chat.ID, update.Message.From)
	case commandReferral:
		s.sendReferral(update.Message.Chat.ID)
	}
}

func (s *Service) handleCallback(ctx context.Context, query *tgbotapi.CallbackQuery) {
	if query == nil || query.Message == nil || query.From == nil {
		return
	}
	s.answerCallback(query.ID)

	chatID := query.Message.Chat.ID
	switch query.Data {
	case callbackSubscription:
		s.sendSubscription(ctx, chatID, query.From)
	case callbackActivateTrial:
		s.activateTrial(ctx, chatID, query.From)
	case callbackMyConfig:
		s.sendMyConfig(ctx, chatID, query.From)
	case callbackReferral:
		s.sendReferral(chatID)
	case callbackHelp:
		s.sendHelp(chatID)
	case callbackBackToStart:
		s.sendStartMenu(chatID)
	}
}

func (s *Service) sendStartMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, renderStartMessage())
	msg.ReplyMarkup = startKeyboard()
	s.sendMessage(msg)
}

func (s *Service) sendSubscription(ctx context.Context, chatID int64, from *tgbotapi.User) {
	user, err := s.getOrCreateUser(ctx, from)
	if err != nil {
		s.log.Error("failed to get or create telegram user", "telegram_id", from.ID, "error", err)
		s.sendText(chatID, "Не получилось открыть подписку. Попробуйте еще раз чуть позже.")
		return
	}

	subscription, err := s.subscriptions.GetActive(ctx, user.GetId())
	if err == nil {
		msg := tgbotapi.NewMessage(chatID, renderActiveSubscriptionMessage(subscription))
		msg.ReplyMarkup = backKeyboard()
		s.sendMessage(msg)
		return
	}
	if status.Code(err) != codes.NotFound {
		s.log.Error("failed to get active subscription", "user_id", user.GetId(), "error", err)
		s.sendText(chatID, "Не получилось проверить подписку. Попробуйте еще раз чуть позже.")
		return
	}

	msg := tgbotapi.NewMessage(chatID, renderNoSubscriptionMessage())
	msg.ReplyMarkup = subscriptionKeyboard()
	s.sendMessage(msg)
}

func (s *Service) activateTrial(ctx context.Context, chatID int64, from *tgbotapi.User) {
	user, err := s.getOrCreateUser(ctx, from)
	if err != nil {
		s.log.Error("failed to get or create telegram user", "telegram_id", from.ID, "error", err)
		s.sendText(chatID, "Не получилось активировать trial. Попробуйте еще раз чуть позже.")
		return
	}

	subscription, err := s.subscriptions.StartTrial(ctx, user.GetId())
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			msg := tgbotapi.NewMessage(chatID, "Trial уже был использован для этого аккаунта. Можно выбрать платный тариф в разделе подписки.")
			msg.ReplyMarkup = backKeyboard()
			s.sendMessage(msg)
			return
		}
		s.log.Error("failed to start trial", "user_id", user.GetId(), "error", err)
		s.sendText(chatID, "Не получилось активировать trial. Попробуйте еще раз чуть позже.")
		return
	}

	configLink, err := s.provisionSubscriptionConfig(ctx, user.GetId(), subscription)
	if err != nil {
		s.log.Error("failed to provision trial config", "user_id", user.GetId(), "subscription_id", subscription.GetId(), "error", err)
		msg := tgbotapi.NewMessage(chatID, renderTrialActivatedMessage(subscription)+"\n\nКонфиг создается с задержкой. Откройте «Мои конфиги» чуть позже.")
		msg.ReplyMarkup = backKeyboard()
		s.sendMessage(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, renderTrialActivatedMessage(subscription)+"\n\n"+renderConfigLinkMessage(configLink))
	msg.ReplyMarkup = backKeyboard()
	s.sendMessage(msg)
}

func (s *Service) getOrCreateUser(ctx context.Context, from *tgbotapi.User) (*userv1.User, error) {
	if from == nil {
		return nil, errors.New("telegram user is nil")
	}

	return s.users.GetOrCreateTelegramUser(ctx, userclient.TelegramUserInput{
		TelegramID:   from.ID,
		Username:     from.UserName,
		FirstName:    from.FirstName,
		LastName:     from.LastName,
		LanguageCode: from.LanguageCode,
	})
}

func (s *Service) sendMyConfig(ctx context.Context, chatID int64, from *tgbotapi.User) {
	user, err := s.getOrCreateUser(ctx, from)
	if err != nil {
		s.log.Error("failed to get or create telegram user", "telegram_id", telegramUserID(from), "error", err)
		s.sendText(chatID, "Не получилось открыть конфиги. Попробуйте еще раз чуть позже.")
		return
	}

	subscription, err := s.subscriptions.GetActive(ctx, user.GetId())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			msg := tgbotapi.NewMessage(chatID, "Активной подписки пока нет. Сначала активируйте trial или тариф в разделе «Подписка».")
			msg.ReplyMarkup = subscriptionKeyboard()
			s.sendMessage(msg)
			return
		}
		s.log.Error("failed to get active subscription", "user_id", user.GetId(), "error", err)
		s.sendText(chatID, "Не получилось проверить подписку. Попробуйте еще раз чуть позже.")
		return
	}

	configLink, err := s.provisionSubscriptionConfig(ctx, user.GetId(), subscription)
	if err != nil {
		s.log.Error("failed to provision config", "user_id", user.GetId(), "subscription_id", subscription.GetId(), "error", err)
		s.sendText(chatID, "Не получилось создать конфиг. Попробуйте еще раз чуть позже.")
		return
	}

	msg := tgbotapi.NewMessage(chatID, renderConfigLinkMessage(configLink))
	msg.ReplyMarkup = backKeyboard()
	s.sendMessage(msg)
}

func (s *Service) provisionSubscriptionConfig(ctx context.Context, userID string, subscription *subscriptionv1.Subscription) (string, error) {
	if s == nil || s.configs == nil {
		return "", errors.New("config-service client is nil")
	}

	config, err := s.configs.ProvisionSubscription(ctx, configclient.ProvisionSubscriptionInput{
		UserID:       userID,
		Subscription: subscription,
		ClientType:   "happ",
		Format:       "xray-json",
	})
	if err != nil {
		return "", err
	}

	return config.SubscriptionURL, nil
}

func telegramUserID(from *tgbotapi.User) int64 {
	if from == nil {
		return 0
	}
	return from.ID
}

func (s *Service) sendReferral(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Реферальная программа появится позже. Сейчас фокусируемся на стабильном подключении и подписке.")
	msg.ReplyMarkup = backKeyboard()
	s.sendMessage(msg)
}

func (s *Service) sendHelp(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, renderHelpMessage())
	msg.ReplyMarkup = helpKeyboard()
	s.sendMessage(msg)
}

func (s *Service) sendText(chatID int64, text string) {
	s.sendMessage(tgbotapi.NewMessage(chatID, text))
}

func (s *Service) sendMessage(msg tgbotapi.MessageConfig) {
	if _, err := s.bot.Send(msg); err != nil {
		s.log.Error("failed to send telegram message", "chat_id", msg.ChatID, "error", err)
	}
}

func (s *Service) answerCallback(callbackID string) {
	if strings.TrimSpace(callbackID) == "" {
		return
	}
	if _, err := s.bot.Request(tgbotapi.NewCallback(callbackID, "")); err != nil {
		s.log.Error("failed to answer telegram callback", "callback_id", callbackID, "error", err)
	}
}

func renderStartMessage() string {
	return strings.Join([]string{
		"Привет от The Dark Twenties.",
		"",
		"Обход блокировок и белых списков.",
		"Умная маршрутизация: российские сервисы работают без отключения VPN.",
		"Поддержка iOS, Android и ПК.",
		"",
		"Выберите действие в меню ниже.",
	}, "\n")
}

func renderHelpMessage() string {
	return strings.Join([]string{
		"Команды:",
		"",
		"/start - главное меню и тарифы",
		"/my_config - мои подключения",
		"/referral - пригласить друга",
		"/help - справка и инструкция",
		"",
		"Как подключиться:",
		"1. Откройте раздел подписки и активируйте trial или тариф.",
		"2. После появления config-service в разделе /my_config будет ссылка на подписку.",
		"3. Импортируйте ссылку в Happ/v2RayTun и включите подключение.",
	}, "\n")
}

func renderNoSubscriptionMessage() string {
	return "Активной подписки пока нет. Можно активировать trial и вернуться к настройке подключения."
}

func renderActiveSubscriptionMessage(subscription *subscriptionv1.Subscription) string {
	if subscription == nil {
		return renderNoSubscriptionMessage()
	}
	return fmt.Sprintf(
		"Подписка активна.\nСтатус: %s\nДействует до: %s",
		subscription.GetStatus(),
		formatTimestamp(subscription.GetExpiresAt()),
	)
}

func renderTrialActivatedMessage(subscription *subscriptionv1.Subscription) string {
	return fmt.Sprintf("Trial активирован.\nДействует до: %s", formatTimestamp(subscription.GetExpiresAt()))
}

func renderConfigLinkMessage(subscriptionURL string) string {
	return strings.Join([]string{
		"Ваш конфиг готов.",
		"",
		"Ссылка для импорта:",
		strings.TrimSpace(subscriptionURL),
		"",
		"Импортируйте ее в Happ/v2RayTun и включите подключение.",
	}, "\n")
}

func startKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Мои конфиги", callbackMyConfig),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Подписка", callbackSubscription),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Пригласить друга", callbackReferral),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Справка и инструкция", callbackHelp),
		),
	)
}

func subscriptionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Активировать trial", callbackActivateTrial),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Назад", callbackBackToStart),
		),
	)
}

func helpKeyboard() tgbotapi.InlineKeyboardMarkup {
	return backKeyboard()
}

func backKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Назад", callbackBackToStart),
		),
	)
}

func formatTimestamp(value interface{ AsTime() time.Time }) string {
	if value == nil {
		return "неизвестно"
	}
	return value.AsTime().Local().Format("02.01.2006 15:04")
}
