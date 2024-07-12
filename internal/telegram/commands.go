package telegram

import (
	"fmt"
	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"net/mail"
)

type Handler struct {
	UpdateChan      tgbotapi.UpdatesChannel
	Bot             *tgbotapi.BotAPI
	Requests        map[string]func(msg *tgbotapi.Message) bool
	RequestPipeline []string
	CommandState    bool
}

func NewHandler(bot *tgbotapi.BotAPI) (*Handler, error) {
	h := &Handler{
		Bot:          bot,
		CommandState: true,
	}

	updateConfig := tgbotapi.NewUpdate(0)
	var err error
	h.UpdateChan, err = h.Bot.GetUpdatesChan(updateConfig)
	if err != nil {
		logrus.Errorf("failed to set updates from telegram bot, error: %v", err)
		return nil, err
	}

	h.InitHandlers()

	return h, nil
}

func (h *Handler) InitHandlers() {
	h.Requests = make(map[string]func(msg *tgbotapi.Message) bool)

	h.Requests["email"] = func(msg *tgbotapi.Message) bool {
		_, err := mail.ParseAddress(msg.Text)
		if err != nil {
			return false
		}
		return true
	}

	h.Requests["password"] = func(msg *tgbotapi.Message) bool {
		if len(msg.Text) < 4 || len(msg.Text) > 20 {
			return false
		}

		for _, elem := range msg.Text {
			if !((elem >= 'a' && elem <= 'z') || (elem >= 'A' && elem <= 'Z') || (elem >= '0' && elem <= '9')) {
				return false
			}
		}

		return true
	}
}

func (h *Handler) GetUpdate() {
	for update := range h.UpdateChan {

		if h.CommandState {
			switch update.Message.Text {

			case "/start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "я бот для знакомств")
				_, err := h.Bot.Send(msg)
				if err != nil {
					logrus.Errorf("failed to send message to chat %v, errorL %v", update.Message.Chat.ID, err)
				}

			case "/register":
				h.disableCommandState()
				h.RequestPipeline = []string{"email", "password"}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "введите ваш емэйл")
				_, err := h.Bot.Send(msg)
				if err != nil {
					logrus.Errorf("failed to send message to chat %v, errorL %v", update.Message.Chat.ID, err)
				}
			}
			continue
		}

		fn := h.nextRequestInPipeline()
		if fn == nil {
			h.enableCommandState()
			continue
		}

		if !fn(update.Message) {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "возникла ошибка, процесс остановлен")
			_, err := h.Bot.Send(msg)
			if err != nil {
				logrus.Errorf("failed to send message to chat %v, errorL %v", update.Message.Chat.ID, err)
			}
		}

		h.RequestPipeline = h.RequestPipeline[1:]
		if len(h.RequestPipeline) == 0 {
			h.enableCommandState()
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("пожалуйста, введите ваш %s", h.RequestPipeline[0]))
		_, err := h.Bot.Send(msg)
		if err != nil {
			logrus.Errorf("failed to send message to chat %v, errorL %v", update.Message.Chat.ID, err)
		}
	}
}

func (h *Handler) nextRequestInPipeline() func(msg *tgbotapi.Message) bool {
	if len(h.RequestPipeline) == 0 {
		h.enableCommandState()
		return nil
	}

	request := h.RequestPipeline[0]
	fn, ok := h.Requests[request]
	if !ok {
		logrus.Error("such request is not exist")
		return nil
	}

	return fn
}

func (h *Handler) disableCommandState() {
	h.CommandState = false
}

func (h *Handler) enableCommandState() {
	h.CommandState = true
}
