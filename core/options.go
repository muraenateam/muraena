package core

import (
	"errors"
	"flag"
)

type Options struct {
	Debug          *bool
	Version        *bool
	NoColors       *bool
	ConfigFilePath *string
}

var ErrInterrupt = errors.New("^C")

func ParseOptions() (Options, error) {
	o := Options{
		ConfigFilePath: flag.String("config", "", "JSON configuration file path"),
		Debug:          flag.Bool("debug", false, "Print debug messages."),
		Version:        flag.Bool("version", false, "Print current version."),
		NoColors:       flag.Bool("no-colors", false, "Disable output color effects."),
	}

	flag.Parse()

	return o, nil
}
