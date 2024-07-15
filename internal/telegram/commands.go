package telegram

import (
	"errors"
	"fmt"
	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net/mail"
)

var (
	passwordErrorLength = errors.New("incorrect length")
	passwordErrorSymbol = errors.New("incorrect symbol")
	emailError          = errors.New("incorrect email")
)

type Handler struct {
	C         <-chan []byte
	eventChan chan []byte

	updateChan    tgbotapi.UpdatesChannel
	Bot           *tgbotapi.BotAPI
	requests      map[string]func(msg *tgbotapi.Message) error
	requestParams requestParams
	chatId        int64
	commandState  bool
}

type requestParams struct {
	requestPipeline []string
	retries         uint8
	retriesMax      uint8
}

func NewHandler(bot *tgbotapi.BotAPI) (*Handler, error) {
	eventChan := make(chan []byte)

	h := &Handler{
		eventChan:    eventChan,
		C:            eventChan,
		Bot:          bot,
		commandState: true,
		requestParams: requestParams{
			retriesMax: uint8(viper.GetInt("telegram.configuration.request_params.retries_count")),
		},
	}

	updateConfig := tgbotapi.NewUpdate(0)
	var err error
	h.updateChan, err = h.Bot.GetUpdatesChan(updateConfig)
	if err != nil {
		logrus.Errorf("failed to set updates from telegram bot, error: %v", err)
		return nil, err
	}

	h.initHandlers()

	return h, nil
}

func (h *Handler) initHandlers() {
	h.requests = make(map[string]func(msg *tgbotapi.Message) error)

	h.requests["email"] = func(msg *tgbotapi.Message) error {
		_, err := mail.ParseAddress(msg.Text)
		if err != nil {
			return emailError
		}

		h.eventChan <- []byte(msg.Text)
		return nil
	}

	passwordMinimumLength := viper.GetInt("telegram.configuration.register.password.minimum_length")
	passwordMaximumLength := viper.GetInt("telegram.configuration.register.password.maximum_length")
	h.requests["password"] = func(msg *tgbotapi.Message) error {
		if len(msg.Text) < passwordMinimumLength || len(msg.Text) > passwordMaximumLength {
			return passwordErrorLength
		}

		for _, elem := range msg.Text {
			if !((elem >= 'a' && elem <= 'z') || (elem >= 'A' && elem <= 'Z') || (elem >= '0' && elem <= '9')) {
				return passwordErrorSymbol
			}
		}

		h.eventChan <- []byte(msg.Text)
		return nil
	}
}

func (h *Handler) GetUpdate() {
	for update := range h.updateChan {

		if h.commandState {
			switch update.Message.Text {
			case "/start":
				_ = h.sendMessageToUser(update.Message.Chat.ID, "I am bot for meetings")

			case "/register":
				h.disableCommandState()
				h.setRequestPipeline("email", "password")
				h.chatId = update.Message.Chat.ID
				_ = h.sendMessageToUser(update.Message.Chat.ID, "starting register procedure")
				h.sendRequestMessageToUser(update.Message.Chat.ID, h.requestParams.requestPipeline[0])

			default:
				_ = h.sendMessageToUser(update.Message.Chat.ID, "such command is not exist")
			}
			continue
		}

		fn := h.getRequestFromPipeline()
		if fn == nil {
			logrus.Warnf("request is not exist in handler, requst: %v", h.requestParams.requestPipeline[0])
			h.setCommandState()
			continue
		}

		if err := fn(update.Message); err != nil {
			h.requestParams.retries++
			if h.requestParams.retries < h.requestParams.retriesMax {
				_ = h.sendMessageToUser(update.Message.Chat.ID, fmt.Sprintf("error occured: %v , try again", err))
				continue
			}

			h.setCommandState()
			_ = h.sendMessageToUser(update.Message.Chat.ID, "too many wrong attempts")
			continue
		}

		if !h.newRequestFromPipeline() {
			h.setCommandState()
			continue
		}

		h.sendRequestMessageToUser(update.Message.Chat.ID, h.requestParams.requestPipeline[0])
	}
}

func (h *Handler) getRequestFromPipeline() func(msg *tgbotapi.Message) error {
	request := h.requestParams.requestPipeline[0]
	fn, ok := h.requests[request]
	if !ok {
		return nil
	}

	return fn
}

func (h *Handler) newRequestFromPipeline() bool {
	h.requestParams.requestPipeline = h.requestParams.requestPipeline[1:]
	if len(h.requestParams.requestPipeline) == 0 {
		return false
	}

	return true
}

func (h *Handler) disableCommandState() {
	h.commandState = false
}

func (h *Handler) enableCommandState() {
	h.commandState = true
}

func (h *Handler) setCommandState() {
	h.enableCommandState()
	h.requestParams.requestPipeline = nil
	h.requestParams.retries = 0
	h.chatId = 0
}

func (h *Handler) setRequestPipeline(request ...string) {
	for _, r := range request {
		h.requestParams.requestPipeline = append(h.requestParams.requestPipeline, r)
	}
}

func (h *Handler) sendMessageToUser(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)

	_, err := h.Bot.Send(msg)
	if err != nil {
		logrus.Errorf("failed to send message to user, error: %v", err)
		return err
	}

	return nil
}

func (h *Handler) sendRequestMessageToUser(chatId int64, message string) {
	msg := "please input your " + message
	_ = h.sendMessageToUser(chatId, msg)
}
