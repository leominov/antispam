package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

const defaultTimeout = 5 * time.Minute

var TokenFlag = flag.String("token", "", "Telegram Bot API token")

type AntispamBot struct {
	Token       string
	Bot         *tgbotapi.BotAPI
	SpamUserIDs map[int]int
	UserMap     map[int]time.Time
}

func NewBot(token string) *AntispamBot {
	return &AntispamBot{
		Token:       token,
		SpamUserIDs: map[int]int{},
		UserMap:     map[int]time.Time{},
	}
}

func (a *AntispamBot) Configure() error {
	if len(a.Token) == 0 {
		return errors.New("Token must be specified")
	}
	bot, err := tgbotapi.NewBotAPI(a.Token)
	if err != nil {
		return fmt.Errorf("Configure: NewBotAPI: %v", err)
	}
	a.Bot = bot
	log.Printf("Configure: Authorized on account: %s", a.Bot.Self.UserName)
	return nil
}

func (a *AntispamBot) GC() {
	for {
		for userID, date := range a.UserMap {
			if time.Now().Sub(date) > defaultTimeout {
				log.Printf("Delete user %d by timeout...", userID)
				delete(a.UserMap, userID)
			}
		}
		time.Sleep(time.Minute)
	}
}

func (a *AntispamBot) HandleIn(message *tgbotapi.Message) {
	user := message.NewChatMember
	log.Printf("Welcome new user %d...", user.ID)
	a.UserMap[user.ID] = time.Now()
}

func (a *AntispamBot) HandleOut(message *tgbotapi.Message) {
	user := message.LeftChatMember
	log.Printf("Goodbye user %d...", user.ID)
	delete(a.UserMap, user.ID)
}

func (a *AntispamBot) HandleMessage(message *tgbotapi.Message) {
	a.IncSpamCounter(message.From)
	log.Printf("SPAM: user=%v message=%s", message.From, message.Text)
	msg := tgbotapi.NewMessage(message.Chat.ID, "Is it spam?")
	a.Bot.Send(msg)
}

func (a *AntispamBot) IsSpamMessage(message *tgbotapi.Message) bool {
	user := message.From
	if message.ForwardDate == 0 {
		return false
	}
	date, ok := a.UserMap[user.ID]
	if !ok {
		return false
	}
	if time.Now().Sub(date) <= defaultTimeout {
		log.Printf("Spam from user %d...", user.ID)
		return true
	}
	return false
}

func (a *AntispamBot) IncSpamCounter(user *tgbotapi.User) {
	log.Printf("Increase spam counter for user %d...", user.ID)
	counter, ok := a.SpamUserIDs[user.ID]
	if !ok {
		counter = 0
	}
	counter++
	a.SpamUserIDs[user.ID] = counter
}

func (a *AntispamBot) Start() error {
	go a.GC()
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := a.Bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	for update := range updates {
		if update.Message == nil || update.Message.From == nil {
			continue
		}
		if update.Message.NewChatMember != nil {
			a.HandleIn(update.Message)
		}
		if update.Message.LeftChatMember != nil {
			a.HandleOut(update.Message)
		}
		if a.IsSpamMessage(update.Message) {
			a.HandleMessage(update.Message)
		}
	}
	return nil
}

func main() {
	var err error
	flag.Parse()
	log.Print("Starting anti-spam bot...")
	bot := NewBot(*TokenFlag)
	err = bot.Configure()
	if err != nil {
		log.Print(err)
		os.Exit(2)
	}
	err = bot.Start()
	if err != nil {
		log.Print(err)
		os.Exit(2)
	}
}
