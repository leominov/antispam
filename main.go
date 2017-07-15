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

const (
	defaultUserTimeout = 5 * time.Minute
	defaultGcInterval  = time.Minute
)

var (
	TokenFlag     = flag.String("token", "", "Telegram Bot API token")
	WatchOnlyFlag = flag.Bool("watch", true, "Only watching")
)

type AntispamBot struct {
	WatchOnly        bool
	Token            string
	Bot              *tgbotapi.BotAPI
	BotUpdateConfig  tgbotapi.UpdateConfig
	UserSpamCounters map[int]int
	UserMap          map[int]time.Time
}

func NewBot(token string, watchOnly bool) *AntispamBot {
	return &AntispamBot{
		WatchOnly:        watchOnly,
		Token:            token,
		UserSpamCounters: map[int]int{},
		UserMap:          map[int]time.Time{},
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
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	a.BotUpdateConfig = u
	log.Printf("Configure: Authorized on account: %s", a.Bot.Self.UserName)
	log.Printf("Configure: Watch only: %v", a.WatchOnly)
	return nil
}

func (a *AntispamBot) GC() {
	log.Printf("Starting gc with %s interval...", defaultGcInterval)
	for {
		for userID, date := range a.UserMap {
			if time.Now().Sub(date) > defaultUserTimeout {
				log.Printf("Delete user %d by timeout...", userID)
				delete(a.UserMap, userID)
			}
		}
		time.Sleep(defaultGcInterval)
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

func (a *AntispamBot) HandleSpamMessage(message *tgbotapi.Message) {
	a.IncreaseUserSpamCounter(message.From)
	log.Printf("SPAM: user=%v message=%s", message.From, message.Text)
	if !a.WatchOnly {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Is it spam?")
		if _, err := a.Bot.Send(msg); err != nil {
			fmt.Printf("Send message error: %v", err)
		}
	}
}

func (a *AntispamBot) IsItSpamMessage(message *tgbotapi.Message) bool {
	user := message.From
	if message.ForwardDate == 0 {
		return false
	}
	if len(message.Text) == 0 {
		return false
	}
	date, ok := a.UserMap[user.ID]
	if !ok {
		return false
	}
	if time.Now().Sub(date) <= defaultUserTimeout {
		log.Printf("[timeout] Spam from user %d...", user.ID)
		return true
	}
	return false
}

func (a *AntispamBot) IncreaseUserSpamCounter(user *tgbotapi.User) {
	log.Printf("Increase spam counter for user %d...", user.ID)
	counter, ok := a.UserSpamCounters[user.ID]
	if !ok {
		counter = 0
	}
	counter++
	a.UserSpamCounters[user.ID] = counter
}

func (a *AntispamBot) IsItMessage(message *tgbotapi.Message) bool {
	if message == nil || message.From == nil {
		return false
	}
	return true
}

func (a *AntispamBot) HandleUpdate(update tgbotapi.Update) {
	message := update.Message
	if !a.IsItMessage(message) {
		return
	}
	if message.NewChatMember != nil {
		a.HandleIn(message)
		return
	}
	if message.LeftChatMember != nil {
		a.HandleOut(message)
		return
	}
	if a.IsItSpamMessage(message) {
		a.HandleSpamMessage(message)
	}
}

func (a *AntispamBot) Start() error {
	go a.GC()
	updates, err := a.Bot.GetUpdatesChan(a.BotUpdateConfig)
	if err != nil {
		return err
	}
	for update := range updates {
		a.HandleUpdate(update)
	}
	return nil
}

func realMain() int {
	var err error
	flag.Parse()
	log.Print("Starting anti-spam bot...")
	bot := NewBot(*TokenFlag, *WatchOnlyFlag)
	err = bot.Configure()
	if err != nil {
		log.Print(err)
		return 2
	}
	err = bot.Start()
	if err != nil {
		log.Print(err)
		return 2
	}
	return 0
}

func main() {
	os.Exit(realMain())
}
