package core

import (
	"flag"
)

type Options struct {
	Verbose        *bool
	Debug          *bool
	Proxy          *bool
	Version        *bool
	NoColors       *bool
	ConfigFilePath *string
}

//var ErrInterrupt = errors.New("^C")

func ParseOptions() (Options, error) {
	o := Options{
		ConfigFilePath: flag.String("config", "", "Path to config file."),
		Verbose:        flag.Bool("verbose", false, "Print verbose messages."),
		Debug:          flag.Bool("debug", false, "Print debug messages."),
		Proxy:          flag.Bool("proxy", false, "Enable internal proxy."),
		Version:        flag.Bool("version", false, "Print current version."),
		NoColors:       flag.Bool("no-colors", false, "Disable output color effects."),
	}

	flag.Parse()

	return o, nil
}
