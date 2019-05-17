package session

import (
	"fmt"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/log"
)

type Module interface {
	Name() string
	Description() string
	Author() string
	Prompt(what string)
}

type SessionModule struct {
	Session *Session
	Name    string
	tag     string
}

func AsTag(name string) string {
	return fmt.Sprintf("[%s] ", tui.Wrap(tui.BACKLIGHTBLUE, tui.Wrap(tui.FOREBLACK, name)))
}

func NewSessionModule(name string, s *Session) SessionModule {
	m := SessionModule{
		Name:    name,
		Session: s,
		tag:     AsTag(name),
	}

	return m
}

func (m *SessionModule) Debug(format string, args ...interface{}) {
	log.Debug(m.tag+format, args...)
}

func (m *SessionModule) Info(format string, args ...interface{}) {
	log.Info(m.tag+format, args...)
}

func (m *SessionModule) Important(format string, args ...interface{}) {
	log.Important(m.tag+format, args...)
}

func (m *SessionModule) Warning(format string, args ...interface{}) {
	log.Warning(m.tag+format, args...)
}

func (m *SessionModule) Error(format string, args ...interface{}) {
	log.Error(m.tag+format, args...)
}

func (m *SessionModule) Fatal(format string, args ...interface{}) {
	log.Fatal(m.tag+format, args...)
}
