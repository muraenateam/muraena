package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/evilsocket/islazy/tui"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

const (
	Name        = "telegram"
	Description = "A module that sends notifications via Telegram chat"
	Author      = "Muraena Team"
)

// Telegram module
type Telegram struct {
	session.SessionModule

	Enabled  bool
	BotToken string
	ChatID   []string
}

// Name returns the module name
func (module *Telegram) Name() string {
	return Name
}

// Description returns the module description
func (module *Telegram) Description() string {
	return Description
}

// Author returns the module author
func (module *Telegram) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *Telegram) Prompt() {

	menu := []string{
		"show",
	}
	result, err := session.DoModulePrompt(Name, menu)
	if err != nil {
		return
	}

	switch result {
	case "show":
		module.PrintConfig()
	}
}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *Telegram, err error) {

	m = &Telegram{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       s.Config.Telegram.Enabled,
		BotToken:      s.Config.Telegram.BotToken,
		ChatID:        s.Config.Telegram.ChatIDs,
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	return
}

func Self(s *session.Session) *Telegram {

	m, err := s.Module(Name)
	if err != nil {
		log.Error("%s", err)
	} else {
		mod, ok := m.(*Telegram)
		if ok {
			return mod
		}
	}

	return nil
}

// PrintConfig shows the actual Telegram configuration
func (module *Telegram) PrintConfig() {
	module.Info("Telegram config:\n\tBotToken: %s\n\tChatIDs:%v", module.BotToken, module.ChatID)
}

func (module *Telegram) getUrl() string {
	return fmt.Sprintf("https://api.telegram.org/bot%s", module.BotToken)
}

func (module *Telegram) Send(message string) {

	if !module.Enabled {
		return
	}

	for _, chat := range module.ChatID {
		if err := module.sendToChat(chat, message); err != nil {
			module.Warning("Message %s was not delivered to chat:%s", tui.Bold(message), tui.Bold(chat))
			module.Debug("%s", tui.Red(err.Error()))
		}
	}
}

func (module *Telegram) sendToChat(chat, message string) (err error) {

	var response *http.Response

	//module.Debug(`curl https://api.telegram.org/bot%s/sendMessage -H "Content-Type: application/json" -v -d '{"chat_id":"%s","text":"%s"}'`, module.BotToken, chat, message)

	// Send the message
	url := fmt.Sprintf("%s/sendMessage", module.getUrl())
	body, _ := json.Marshal(map[string]string{
		"chat_id": chat,
		"text":    message,
	})
	response, err = http.Post(
		url,
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	// Close the request at the end
	defer response.Body.Close()

	// Body
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	module.Debug("%s", string(body))
	return
}
