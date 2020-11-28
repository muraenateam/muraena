package log

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	ll "github.com/evilsocket/islazy/log"
	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core"
)

type logger struct {
	Writer       *os.File
	Level        ll.Verbosity
	FormatConfig FormatConfig
	NoEffects    bool
}

var (
	lock      = &sync.Mutex{}
	loggers   = map[string]logger{}
	reEffects = []*regexp.Regexp{
		regexp.MustCompile("\x033\\[\\d+m"),
		regexp.MustCompile("\\\\e\\[\\d+m"),
		regexp.MustCompile("\x1b\\[\\d+m"),
	}
)

func Init(opt core.Options, isLogToFile bool, logFilePath string) {

	noEffects := false
	if !tui.Effects() {
		noEffects = true
		if *opt.NoColors {
			fmt.Printf("\n\nWARNING: Terminal colors have been disabled, view will be very limited.\n\n")
		} else {
			fmt.Printf("\n\nWARNING: This terminal does not support colors, view will be very limited.\n\n")
		}
	}

	logLevel := ll.INFO
	if *opt.Debug {
		logLevel = ll.DEBUG
	}

	config := FormatConfigBasic
	config.Format = "{datetime} {level:color}{level:name}{reset} {message}"
	// Console Log
	err := AddOutput("", logLevel, config, noEffects)
	if err != nil {
		panic(err)
	}

	// File Log
	if isLogToFile && logFilePath != "" {
		err = AddOutput(logFilePath, logLevel, config, true)
		if err != nil {
			panic(err)
		}
	}
}

func AddOutput(path string, level ll.Verbosity, config FormatConfig, noEffects bool) (err error) {
	var writer *os.File
	if path == "" {
		writer = os.Stdout
	} else {
		writer, err = os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return
		}
	}
	lock.Lock()
	loggers[path] = logger{writer, level, config, noEffects}
	lock.Unlock()
	return
}

func (l *logger) emit(s string) {
	// remove all effects if found
	if l.NoEffects {
		for _, re := range reEffects {
			s = re.ReplaceAllString(s, "")
		}
	}

	s = strings.Replace(s, "%", "%%", -1)
	if _, err := fmt.Fprintf(l.Writer, s+string("\n")); err != nil {
		fmt.Printf("Emit error: %+v", err)
	}

}

func do(v ll.Verbosity, format string, args ...interface{}) {
	lock.Lock()
	defer lock.Unlock()

	if len(loggers) <= 0 {
		panic("No Output added to log")
	}

	for _, l := range loggers {

		if l.Level > v {
			continue
		}

		currMessage := format
		if args != nil {
			currMessage = fmt.Sprintf(format, args...)
		}

		tokens := map[string]func() string{
			"{date}": func() string {
				return time.Now().Format(l.FormatConfig.DateFormat)
			},
			"{time}": func() string {
				return time.Now().Format(l.FormatConfig.TimeFormat)
			},
			"{datetime}": func() string {
				return time.Now().Format(l.FormatConfig.DateTimeFormat)
			},
			"{level:value}": func() string {
				return strconv.Itoa(int(v))
			},
			"{level:name}": func() string {
				return ll.LevelNames[v]
			},
			"{level:color}": func() string {
				return ll.LevelColors[v]
			},
			"{message}": func() string {
				return currMessage
			},
		}

		logLine := l.FormatConfig.Format

		// process token -> callback
		for token, cb := range tokens {
			logLine = strings.Replace(logLine, token, cb(), -1)
		}
		// process token -> effect
		for token, effect := range Effects {
			logLine = strings.Replace(logLine, token, effect, -1)
		}
		// make sure an user error does not screw the log
		if tui.HasEffect(logLine) && !strings.HasSuffix(logLine, tui.RESET) {
			logLine += tui.RESET
		}

		l.emit(logLine)
	}

}

// Raw emits a message without format to the logs.
func Raw(format string, args ...interface{}) {
	lock.Lock()
	defer lock.Unlock()

	for _, l := range loggers {
		currMessage := fmt.Sprintf(format, args...)
		l.emit(currMessage)
	}
}

// Debug emits a debug message.
func Debug(format string, args ...interface{}) {
	do(ll.DEBUG, format, args...)
}

// Info emits an informative message.
func Info(format string, args ...interface{}) {
	do(ll.INFO, format, args...)
}

// Important emits an important informative message.
func Important(format string, args ...interface{}) {
	do(ll.IMPORTANT, format, args...)
}

// Warning emits a warning message.
func Warning(format string, args ...interface{}) {
	do(ll.WARNING, format, args...)
}

// Error emits an error message.
func Error(format string, args ...interface{}) {
	do(ll.ERROR, format, args...)
}

// Fatal emits a fatal error message and calls the ll.OnFatal callback.
func Fatal(format string, args ...interface{}) {
	do(ll.FATAL, format, args...)
	os.Exit(1)
}
