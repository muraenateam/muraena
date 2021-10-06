package watchdog

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/manifoldco/promptui"

	"github.com/muraenateam/muraena/core"
)

type ResponseCode string

const (
	rNginx404  ResponseCode = "404_nginx"
	rCustom301 ResponseCode = "301_custom"
)

// PromptResponseAction allows to setup the response actions using the interactive prompt
func (module *Watchdog) PromptResponseAction() {

	type Responses struct {
		Code        ResponseCode
		Description string
	}

	responses := []Responses{
		{Code: rNginx404, Description: "Nginx 404 page"},
		{Code: rCustom301, Description: "Page moved permanently"},
	}

	prompt := promptui.Select{
		Label: "Select the response action to use:",
		Items: responses,
	}

	id, _, err := prompt.Run()
	if err != nil {
		module.Error("Prompt failed %v\n", err)
		return
	}

	responseAction := responses[id]
	switch responseAction.Code {

	case rNginx404:
		module.Action = ResponseAction{Code: responseAction.Code}

	case rCustom301:
		p := promptui.Prompt{
			Label: "Enter target URL",
		}

		targetURL, err := p.Run()
		if core.IsError(err) {
			module.Warning("%v+\n", err)
			return
		}

		module.Action = ResponseAction{Code: responseAction.Code, TargetURL: targetURL}
	}
}

// BlockRequest takes action and send the visitor to a chosen destination, i.e. blocks or trolls him
func (module *Watchdog) CustomResponse(response http.ResponseWriter, request *http.Request) {

	switch module.Action.Code {

	case rNginx404:
		module.NginxNotFound(response, request)

	case rCustom301:
		module.CustomMovedPermanently(response, request, module.Action.TargetURL)
	}
}

// NginxNotFound replies with a 404 page similar to nginx server
func (module *Watchdog) NginxNotFound(w http.ResponseWriter, r *http.Request) {

	var body = `<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx/1.15.5 (Ubuntu)</center>
</body>
</html>
<!-- a padding to disable MSIE and Chrome friendly error page -->
<!-- a padding to disable MSIE and Chrome friendly error page -->
<!-- a padding to disable MSIE and Chrome friendly error page -->
<!-- a padding to disable MSIE and Chrome friendly error page -->
<!-- a padding to disable MSIE and Chrome friendly error page -->
<!-- a padding to disable MSIE and Chrome friendly error page -->`

	headers := w.Header()
	headers.Set("Server", "nginx/1.15.5 (Ubuntu)")
	headers.Set("Content-Type", "text/html")

	type gzipResponseWriter struct {
		io.Writer
		http.ResponseWriter
	}

	var gzr gzipResponseWriter
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		gzr = gzipResponseWriter{Writer: gz, ResponseWriter: w}

		defer func() {
			err := gz.Close()
			if core.IsError(err) {
				module.Err(err)
			}
		}()
	}

	w.WriteHeader(http.StatusNotFound)

	if gzr.Writer != nil {
		_, err := fmt.Fprintf(gzr.Writer, body)
		if core.IsError(err) {
			module.Err(err)
		}

		return
	}

	_, err := fmt.Fprintf(w, body)
	if core.IsError(err) {
		module.Err(err)
	}
}

// CustomMovedPermanently redirects to targetURL page with 301 response header
func (module *Watchdog) CustomMovedPermanently(w http.ResponseWriter, r *http.Request, targetURL string) {
	http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
}
