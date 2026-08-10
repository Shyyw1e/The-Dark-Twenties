package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewServiceDefaults(t *testing.T) {
	service := NewService(nil, nil, nil, nil, config.TelegramConfig{}, nil)

	if service.Name() != serviceName {
		t.Fatalf("name = %q, want %q", service.Name(), serviceName)
	}
	if service.pollTimeout != 30 {
		t.Fatalf("poll timeout = %d, want 30", service.pollTimeout)
	}
	if service.done == nil {
		t.Fatal("done channel is nil")
	}
}

func TestStartValidatesDependencies(t *testing.T) {
	service := NewService(nil, nil, nil, nil, config.TelegramConfig{}, nil)

	err := service.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "telegram bot api is nil") {
		t.Fatalf("error = %v, want telegram bot api validation", err)
	}
}

func TestStartKeyboard(t *testing.T) {
	keyboard := startKeyboard()

	if len(keyboard.InlineKeyboard) != 4 {
		t.Fatalf("keyboard rows = %d, want 4", len(keyboard.InlineKeyboard))
	}
	tests := []struct {
		row      int
		text     string
		callback string
	}{
		{row: 0, text: "Мои конфиги", callback: callbackMyConfig},
		{row: 1, text: "Подписка", callback: callbackSubscription},
		{row: 2, text: "Пригласить друга", callback: callbackReferral},
		{row: 3, text: "Справка и инструкция", callback: callbackHelp},
	}

	for _, tt := range tests {
		button := keyboard.InlineKeyboard[tt.row][0]
		if button.Text != tt.text {
			t.Fatalf("row %d text = %q, want %q", tt.row, button.Text, tt.text)
		}
		if button.CallbackData == nil || *button.CallbackData != tt.callback {
			t.Fatalf("row %d callback = %v, want %q", tt.row, button.CallbackData, tt.callback)
		}
	}
}

func TestSubscriptionKeyboard(t *testing.T) {
	keyboard := subscriptionKeyboard()
	if len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("keyboard rows = %d, want 2", len(keyboard.InlineKeyboard))
	}
	trial := keyboard.InlineKeyboard[0][0]
	if trial.CallbackData == nil || *trial.CallbackData != callbackActivateTrial {
		t.Fatalf("trial callback = %v", trial.CallbackData)
	}
}

func TestRenderMessages(t *testing.T) {
	if strings.Contains(renderStartMessage(), "HappyCoins") {
		t.Fatal("start message must not mention HappyCoins")
	}
	if !strings.Contains(renderHelpMessage(), "/my_config") {
		t.Fatal("help message must mention /my_config")
	}
	if !strings.Contains(renderConfigLinkMessage("https://vpn.example.com/sub/token"), "https://vpn.example.com/sub/token") {
		t.Fatal("config link message must include subscription url")
	}

	expiresAt := time.Date(2026, 8, 6, 12, 30, 0, 0, time.UTC)
	message := renderActiveSubscriptionMessage(&subscriptionv1.Subscription{
		Status:    "active",
		ExpiresAt: timestamppb.New(expiresAt),
	})
	if !strings.Contains(message, "Подписка активна") || !strings.Contains(message, "active") {
		t.Fatalf("active subscription message = %q", message)
	}
}
