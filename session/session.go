package session

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/evilsocket/islazy/log"
	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core"
)

type moduleList []Module

// Session structure
type Session struct {
	Options core.Options   `json:"options"`
	Config  *Configuration `json:"configuration"`
	Modules moduleList     `json:"modules"`
}

// New session
func New() (*Session, error) {
	opts, err := core.ParseOptions()
	if err != nil {
		return nil, err
	}

	if *opts.NoColors || !tui.Effects() {
		tui.Disable()
		log.NoEffects = true
	}

	s := &Session{
		Options: opts,
		Modules: make([]Module, 0),
	}

	fmt.Printf("%s v%s (built for %s %s with %s)\n", core.Name, core.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	if *s.Options.Version {
		os.Exit(0)
	}

	log.Level = log.INFO
	log.Format = "{datetime} {level:color}{level:name}{reset}: {message}"
	if *s.Options.Debug == true {
		log.Level = log.DEBUG
		log.Debug("DEBUG ON")
	}

	ascii, err := ioutil.ReadFile("muraena-ascii.txt")
	if err != nil {
		log.Fatal("Muraena needs its ascii masturbat1on!")
	}
	log.Format = "\n{level:color}{message}{reset}"
	log.Important("%s", tui.Bold(tui.Red(string(ascii))))

	log.Format = "{datetime} {level:color}{level:name}{reset}: {message}"

	// Load the configuration
	if err := s.GetConfiguration(); err != nil {
		return nil, err
	}

	// Load prompt
	go Prompt(s)

	return s, nil
}

// Module retrieves a module from session modules
func (s *Session) Module(name string) (mod Module, err error) {
	for _, m := range s.Modules {
		if m.Name() == name {
			return m, nil
		}
	}

	return nil, fmt.Errorf("module %s not found", name)
}

// Register appends the provided module to the session
func (s *Session) Register(mod Module, err error) {
	if err != nil {
		log.Error(err.Error())
	} else {
		s.Modules = append(s.Modules, mod)
	}
}
