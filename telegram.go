package main

import (
	"context"
	"database/sql"
	"fmt"

	bot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var telegramBot *bot.BotAPI
var geminiKey string
var tgModel string = "gemini-3.1-flash-lite-preview"
var tgReasoning string = "MINIMAL"

func setGeminiKey() error {
	var exists bool
	geminiKey, exists = checkForEnv()

	if !exists {
		fError := fmt.Errorf("No GEMINI KEY was found in environment")
		return fError
	}
	return nil
}

func botClient(key string) error {
	var err error
	telegramBot, err = bot.NewBotAPI(key)
	if err != nil {
		fError := fmt.Errorf("There was an error initializing the telegram client: %v", err)
		return fError
	}
	telegramBot.Debug = true
	return nil
}

func sendMessage(text string, message *bot.Message) {
	chatId := message.Chat.ID
	messageID := message.MessageID

	msg := bot.NewMessage(chatId, text)
	msg.ReplyToMessageID = messageID

	_, err := telegramBot.Send(msg)

	if err != nil {
		fError := fmt.Errorf("Error while sending message to client: %v", err)
		fmt.Println(fError)
	}
}

func commandsHandler(message *bot.Message) {

	commands := message.Command()
	commandsArguments := message.CommandArguments()

	switch commands {
	case "start":
		sendMessage("👋 Welcome to the Gemini Telegram Bot!\nUse /help or /about to see available commands.", message)
	case "help", "about":
		helpText := "📋 *Available Commands* \n\n/start - Show welcome message\n/model [name] - Change the AI model\n/help or /about - Show this help menu\n\nCurrent model: " + tgModel
		sendMessage(helpText, message)
	case "model":
		if commandsArguments == "" {
			// since no arguments were passed, list all the models
			sendMessage(fmt.Sprintf("Available Models are:\n1. gemini-3-flash-preview\n2. gemini-3.1-flash-preview-lite\n3. any model from google\nCurrent model: %s", tgModel), message)
		} else {
			tgModel = resolveModels(commandsArguments)
			sendMessage(fmt.Sprintf("Model changed to: %s", tgModel), message)
		}
	case "reasoning":
		if commandsArguments == "" {
			sendMessage(fmt.Sprintf("Available Reasoning Levels: \n1. HIGH\n2. MEDIUM\n3. LOW\n4. MINIMAL\nCurrent reasoning level: %s", tgReasoning), message)
		} else {
			tgReasoning = resolveReasoningLevel(commandsArguments)
			sendMessage(fmt.Sprintf("Reasoning changed to: %s", tgReasoning), message)
		}

	default:
		sendMessage(fmt.Sprintf("No such commands found: %s", commands), message)
	}
}

func botConfig(ctx context.Context, db *sql.DB) {
	// some configs i copied from https://go-telegram-bot-api.dev
	updateConfig := bot.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := telegramBot.GetUpdatesChan(updateConfig)

	fmt.Println("Alright! Going to listen for events from telegram!")
	for update := range updates {
		message := update.Message

		if message == nil {
			continue
		}

		if message.IsCommand() {
			commandsHandler(message)
			continue
		}

		// the message from the update
		receivedMessage := update.Message.Text
		// my user id
		id := update.Message.Chat.ID
		// the id of the reply
		messageId := update.Message.MessageID

		// make a new message, with my id and the message
		res := runAgentTurn(ctx, db, geminiKey, receivedMessage, tgModel, "MINIMAL", true)

		msg := bot.NewMessage(id, receivedMessage)
		// set the reply to id (idk)
		msg.ReplyToMessageID = messageId

		sendMessage(res, message)

		// saving the message and response to local sqlite database
		saveMessage(db, "user", receivedMessage)
		saveMessage(db, "assistant", res)
	}
}
