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

func GetDefaultOptions() Options {
	return Options{
		Verbose:        &[]bool{false}[0],
		Debug:          &[]bool{false}[0],
		Proxy:          &[]bool{false}[0],
		Version:        &[]bool{false}[0],
		NoColors:       &[]bool{false}[0],
		ConfigFilePath: &[]string{""}[0],
	}
}
