// Package statichttp serves simple HTTP server
package statichttp

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/session"
)

const (
	// Name of this module
	Name = "static.http"

	// Description of this module
	Description = "Exposes a simple HTTP server to serve static resources during the MiTM session"

	// Author of this module
	Author = "Muraena Team"
)

// StaticHTTP module
type StaticHTTP struct {
	session.SessionModule
	config  session.StaticHTTPConfig
	mux     *http.ServeMux `toml:"-"`
	address string         `toml:"-"`
}

// Name returns the module name
func (module *StaticHTTP) Name() string {
	return Name
}

// Description returns the module description
func (module *StaticHTTP) Description() string {
	return Description
}

// Author returns the module author
func (module *StaticHTTP) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *StaticHTTP) Prompt() {
	module.Raw("No options are available for this module")
}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *StaticHTTP, err error) {

	m = &StaticHTTP{
		SessionModule: session.NewSessionModule(Name, s),
		config:        s.Config.StaticServer,
	}

	if !m.config.Enabled {
		m.Debug("is disabled")
		return
	}

	// Enable static server module
	if err = m.start(); err != nil {
		return
	}

	m.Info("I'm alive and kicking at %s", tui.Bold(tui.Green(m.address)))
	return
}

// Debugging wrapper around the file server
func (module *StaticHTTP) logFileServer(handler http.Handler, localPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(localPath, r.URL.Path)
		module.Info("Requested %s", filePath)
		handler.ServeHTTP(w, r)
	}
}

func (module *StaticHTTP) configure() error {
	config := module.config

	// :0 means "assign an available port"
	module.address = fmt.Sprintf("%s:%d", config.ListeningHost, config.ListeningPort)
	module.mux = http.NewServeMux()

	path := http.Dir(config.LocalPath)
	module.Debug("[Static Server] Requested resource: %s", path)

	fileServer := http.FileServer(FileSystem{path})
	debugFS := module.logFileServer(fileServer, config.LocalPath)
	module.mux.Handle(config.URLPath, http.StripPrefix(strings.TrimRight(config.URLPath, "/"), debugFS))
	return nil
}

// start runs the Static HTTP server module
func (module *StaticHTTP) start() (err error) {
	if err = module.configure(); err != nil {
		return
	}

	listener, err := net.Listen("tcp", module.address)
	if err != nil {
		log.Fatalf("Error creating listener: %v", err)
	}

	// Retrieve the actual address & port assigned by the system
	module.address = listener.Addr().String()
	module.Info("listening on %s", module.address)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done() // Signal the server is ready

		if err := http.Serve(listener, module.mux); err != nil {
			module.Error("%v", err)
		}
	}()

	// Wait for the server to start
	wg.Wait()
	return
}

// FileSystem custom file system handler
type FileSystem struct {
	fs http.FileSystem
}

// Open opens file
func (fs FileSystem) Open(path string) (http.File, error) {
	f, err := fs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// GetNewDestination returns the destination URL for the given request
func (module *StaticHTTP) GetNewDestination(URL *url.URL) (destination string) {
	if strings.HasPrefix(URL.Path, module.config.URLPath) {
		destination = fmt.Sprintf("http://%s", module.address)
	}

	return
}
