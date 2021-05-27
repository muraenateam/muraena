package session

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/evilsocket/islazy/tui"
	"github.com/manifoldco/promptui"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

const (
	InvalidCommand = "invalid option, enter help for assistance"
)

func Prompt(s *Session) {

	for {
		templates := &promptui.PromptTemplates{
			Prompt:  "{{ . | }} ",
			Valid:   "{{ . | green }} ",
			Invalid: "{{ . | red }} ",
			Success: "{{ . | bold }} ",
		}

		validate := func(input string) error {
			input = strings.ToLower(input)

			if core.StringContains(input, []string{"", "h", "help", "e", "exit"}) {
				return nil
			}

			if core.StringContains(input, s.GetModuleNames()) {
				return nil
			}

			return errors.New(InvalidCommand)
		}

		prompt := promptui.Prompt{
			Label:     ">",
			Templates: templates,
			Validate:  validate,
		}

		result, err := prompt.Run()
		if err == promptui.ErrInterrupt {
			exit()
		} else if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		result = strings.ToLower(result)

		// Module menu
		if core.StringContains(result, s.GetModuleNames()) {
			// Retrieve module's object
			m, err := s.Module(result)
			if err != nil {
				log.Error("%s", err)
				return
			}

			m.Prompt()

		} else {

			switch result {
			case "h", "help":
				s.help()
			case "e", "exit":
				exit()
			}
		}
	}
}

func (s *Session) help() {
	log.Raw("**************************************************************************")
	log.Raw("* Muraena menu")
	log.Raw("* - h, help: %s", tui.Bold("Prints this help"))
	log.Raw("* - e, exit: %s", tui.Bold("Exit from "+core.Name))
	log.Raw("* Enabled modules:")
	for _, m := range s.GetModuleNames() {
		log.Raw("* - %s: %s", m, tui.Bold("Interact with "+m+" module"))
	}
	log.Raw("**************************************************************************")

}

func exit() {
	prompt := promptui.Prompt{
		Label:     "Do you want to exit",
		IsConfirm: true,
		Default:   "n",
	}
	answer, _ := prompt.Run()
	if strings.ToLower(answer) == "y" {
		os.Exit(0)
	}
}

// DoModulePrompt generates a prompt for a specific module
func DoModulePrompt(module string, items []string) (result string, err error) {

	prompt := promptui.Select{
		Label: module + " actions:",
		Items: items,
	}
	_, result, err = prompt.Run()
	result = strings.ToLower(result)

	//if core.IsError(err) {
	//	log.Debug("%s prompt menu failed: %v.", module, err)
	//}

	return
}
