package log

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core"
)

type logger struct {
	Writer       *os.File
	Level        Verbosity
	FormatConfig FormatConfig
	NoEffects    bool
}

var (
	NoEffects = false
	lock      = &sync.Mutex{}
	loggers   = map[string]logger{}
	reEffects = []*regexp.Regexp{
		regexp.MustCompile("\x033\\[\\d+m"),
		regexp.MustCompile("\\\\e\\[\\d+m"),
		regexp.MustCompile("\x1b\\[\\d+m"),
	}
)

func Init(opt core.Options, isLogToFile bool, logFilePath string) {

	if !tui.Effects() {
		NoEffects = true
		if !*opt.NoColors {
			fmt.Printf("\n\nWARNING: This terminal does not support colors, view will be very limited.\n\n")
		}
	}

	logLevel := INFO
	if *opt.Verbose {
		logLevel = VERBOSE
	} else if *opt.Debug {
		logLevel = DEBUG
	}

	config := FormatConfigBasic
	config.Format = "{datetime} {level:color}{level:name}{reset} {message}"
	// Console Log
	err := AddOutput("", logLevel, config, NoEffects)
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

func AddOutput(path string, level Verbosity, config FormatConfig, NoEffects bool) (err error) {
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
	loggers[path] = logger{writer, level, config, NoEffects}
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

func do(v Verbosity, format string, args ...interface{}) {
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
				return LevelNames[v]
			},
			"{level:color}": func() string {
				return LevelColors[v]
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

// Verbose emits a verbose message.
func Verbose(format string, args ...interface{}) {
	do(VERBOSE, format, args...)
}

// Debug emits a debug message.
func Debug(format string, args ...interface{}) {
	do(DEBUG, format, args...)
}

// Info emits an informative message.
func Info(format string, args ...interface{}) {
	do(INFO, format, args...)
}

// Important emits an important informative message.
func Important(format string, args ...interface{}) {
	do(IMPORTANT, format, args...)
}

// Warning emits a warning message.
func Warning(format string, args ...interface{}) {
	do(WARNING, format, args...)
}

// Error emits an error message.
func Error(format string, args ...interface{}) {
	do(ERROR, format, args...)
}

// Fatal emits a fatal error message and calls the OnFatal callback.
func Fatal(format string, args ...interface{}) {
	do(FATAL, format, args...)
	os.Exit(1)
}
