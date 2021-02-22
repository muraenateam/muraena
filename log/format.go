package log

import (
	"github.com/evilsocket/islazy/tui"
)

type FormatConfig struct {
	DateFormat     string
	TimeFormat     string
	DateTimeFormat string
	Format         string
}

var (
	// Effects is a map of the tokens that can be used in Format to
	// change the properties of the text.
	Effects = map[string]string{
		"{bold}":        tui.BOLD,
		"{dim}":         tui.DIM,
		"{red}":         tui.RED,
		"{green}":       tui.GREEN,
		"{blue}":        tui.BLUE,
		"{yellow}":      tui.YELLOW,
		"{f:black}":     tui.FOREBLACK,
		"{f:white}":     tui.FOREWHITE,
		"{b:darkgray}":  tui.BACKDARKGRAY,
		"{b:red}":       tui.BACKRED,
		"{b:green}":     tui.BACKGREEN,
		"{b:yellow}":    tui.BACKYELLOW,
		"{b:lightblue}": tui.BACKLIGHTBLUE,
		"{reset}":       tui.RESET,
	}

	FormatConfigBasic = FormatConfig{
		DateFormat:     dateFormat,
		TimeFormat:     timeFormat,
		DateTimeFormat: dateTimeFormat,
		Format:         format,
	}
	// dateFormat is the default date format being used when filling the {date} log token.
	dateFormat = "06-Jan-02"
	// timeFormat is the default time format being used when filling the {time} or {datetime} log tokens.
	timeFormat = "15:04:05"

	// RFC822
	// DateTimeFormat is the default date and time format being used when filling the {datetime} log token.
	dateTimeFormat = "02 Jan 06 15:04 MST"
	// Format is the default format being used when logging.
	format = "{datetime} {level:color}{level:name}{reset} {message}"
)
