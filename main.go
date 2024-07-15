package main

import (
	"TelegramBot/internal/telegram"
	"context"
	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

var (
	BotApi *tgbotapi.BotAPI
)

func main() {
	mustInitConfig()
	mustInitTelegramBot()

	initTelegramMessageProcessing()

	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	<-ctx.Done()
}

func mustInitTelegramBot() {
	token := os.Getenv(viper.GetString("telegram.http-api"))

	var err error
	BotApi, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Fatalf("failed to connect to telegram bot, error: %v", err)
	}
}

func initTelegramMessageProcessing() {
	h, err := telegram.NewHandler(BotApi)
	if err != nil {
		logrus.Fatalf("failed to init telegram handler, error: %v", err)
	}

	go func() {
		h.GetUpdate()
	}()
}

func mustInitConfig() {
	viper.SetConfigFile("./configuration.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logrus.Fatalf("failed to read configuration file, error: %v", err)
	}
}
