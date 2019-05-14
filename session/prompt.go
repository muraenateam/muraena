package session

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
)

func Prompt(s *Session) {

	for {
		validate := func(input string) error {

			switch strings.ToLower(input) {
			case
				"", "h",
				"help",
				"exit",
				"sessions":
				return nil
			}

			return errors.New("invalid command, enter help for assistance")
		}

		templates := &promptui.PromptTemplates{
			Prompt:  "{{ . | }} ",
			Valid:   "{{ . | green }} ",
			Invalid: "{{ . | red }} ",
			Success: "{{ . | bold }} ",
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

		switch strings.ToLower(result) {
		case "h", "help":
			help()
		case "exit":
			exit()
		}
	}
}

func help() {
	fmt.Printf("this should be helpful: %s\n", ".")
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
