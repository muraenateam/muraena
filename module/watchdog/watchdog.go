package watchdog

// Parts of this module have been taken from ZeroDrop (https://github.com/oftn-oswg/zerodrop)

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/evilsocket/islazy/tui"
	"github.com/fsnotify/fsnotify"
	"github.com/manifoldco/promptui"
	"github.com/oschwald/geoip2-golang"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/session"
)

const (
	Name        = "watchdog"
	Description = "A module that helps to manage the access control based on rules."
	Author      = "Muraena Team"
)

// Watchdog module
type Watchdog struct {
	session.SessionModule

	Enabled       bool
	Dynamic       bool
	Raw           string
	Rules         Blacklist
	RulesFilePath string
	GeoDB         *geoip2.Reader
	GeoDBFilePath string

	Action ResponseAction
}

// Rule is a structure that represents the rules of a blacklist
type Rule struct {
	Raw       string
	All       bool
	Negation  bool
	IP        net.IP
	Network   *net.IPNet
	Hostname  string
	Regexp    string
	Geofence  *Geofence
	UserAgent string
}

// Blacklist is a list of Rules
type Blacklist struct {
	List []*Rule
}

// ResponseAction contains actions to perform after a block
type ResponseAction struct {
	Code ResponseCode

	// Optional parameters
	TargetURL string
}

// Name returns the module name
func (module *Watchdog) Name() string {
	return Name
}

// Description returns the module description
func (module *Watchdog) Description() string {
	return Description
}

// Author returns the module author
func (module *Watchdog) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *Watchdog) Prompt() {

	menu := []string{
		"rules",
		"flush",
		"reload",
		"save",
		"add",
		"remove",
		"response",
	}
	result, err := session.DoModulePrompt(Name, menu)
	if err != nil {
		return
	}

	switch result {
	case "rules":
		module.PrintRules()

	case "flush":
		module.Flush()

	case "reload":
		module.Reload()

	case "save":
		module.Save()

	case "add":
		prompt := promptui.Prompt{
			Label: "Enter rule to add",
		}

		result, err := prompt.Run()
		if core.IsError(err) {
			module.Warning("%v+\n", err)
			return
		}

		add := module.Rules.AppendRaw(result)
		if add {
			module.Info("New rule: %s", result)
		} else {
			module.Warning("Error adding new rule: %s", result)
		}

	case "remove":
		prompt := promptui.Select{
			Label: "Select rule to remove",
			Items: module.Rules.List,
		}

		i, _, err := prompt.Run()
		if core.IsError(err) {
			module.Warning("%v+\n", err)
			return
		}

		module.Info("Removing rule: %s", module.Rules.List[i].Raw)
		module.Rules.Remove(module.Rules.List[i])
		module.PrintRules()

	case "response":
		module.PromptResponseAction()

	}

}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *Watchdog, err error) {

	m = &Watchdog{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       s.Config.Watchdog.Enabled,
		Dynamic:       s.Config.Watchdog.Dynamic,
		RulesFilePath: s.Config.Watchdog.Rules,
		GeoDBFilePath: s.Config.Watchdog.GeoDB,
	}

	if m.Enabled {
		config := s.Config.Watchdog

		// Parse raw rules
		if _, err := os.Stat(config.Rules); err == nil {
			rules, err := ioutil.ReadFile(config.Rules)
			if err != nil {
				m.Raw = string(rules)
			}
		}

		m.Reload()

		if m.Dynamic {
			go m.MonitorRules()
		}

		//	TODO: make this customizable
		//	 Set default response action to 404 Nginx
		m.Action = ResponseAction{Code: rNginx404}
		return
	}

	m.Debug("is disabled")
	return
}

// Reload reparses the rules to update the Blacklist
func (module *Watchdog) Reload() {
	module.loadRules()
	module.loadGeoDB()
	module.Info("Watchdog rules reloaded successfully")
}

// Flush removes all the rules
func (module *Watchdog) Flush() {
	module.Raw = ""
	module.Rules = Blacklist{List: []*Rule{}}
	module.Info("Watchdog rules flushed successfully")
}

// PrintRules pretty prints the list of active rules
func (module *Watchdog) PrintRules() {
	module.Info("Watchdog rules:")
	module.Info("%s", module.getRulesString())
}

// Save dumps current Blacklist to file
func (module *Watchdog) Save() {

	rules := module.getRulesString()

	f, err := os.OpenFile(module.RulesFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if core.IsError(err) {
		module.Err(err)
		return
	}

	if err = f.Truncate(0); core.IsError(err) {
		module.Err(err)
		return
	}

	if _, err = f.WriteString(rules); core.IsError(err) {
		module.Err(err)
		return
	}

	if err := f.Close(); core.IsError(err) {
		module.Err(err)
		return
	}
}

func (module *Watchdog) getRulesString() string {
	rules := ""
	for _, rule := range module.Rules.List {
		rules += rule.Raw + " \n"
	}

	return rules
}

func (module *Watchdog) loadRules() {

	if module.RulesFilePath != "" {
		module.Debug("Loading rules at %s", module.RulesFilePath)

		if _, err := os.Stat(module.RulesFilePath); err == nil {
			rules, err := ioutil.ReadFile(module.RulesFilePath)
			if err != nil {
				module.Error(err.Error())
				return
			}

			module.Raw = string(rules)
		}
	}

	// Parse rules
	module.Rules = ParseRules(module.Raw)
	module.Debug("%d parsed rules.", len(module.Rules.List))
	return
}

func (module *Watchdog) loadGeoDB() {

	if module.GeoDBFilePath == "" {
		return
	}

	var err error
	module.GeoDB, err = geoip2.Open(module.GeoDBFilePath)
	if core.IsError(err) {
		module.Warning("Could not open geolocation database: %s", err.Error())
	}

	return
}

// Add appends a Rule to the Blacklist
func (b *Blacklist) Add(item *Rule) {
	b.List = append(b.List, item)
}

// Remove removes a Rule from the Blacklist
func (b *Blacklist) Remove(item *Rule) bool {
	for i := range b.List {
		if b.List[i] == item {
			b.List = append(b.List[:i], b.List[i+1:]...)
			return true
		}
	}
	return false
}

// AppendRaw parse a rule string and appends the Rule to the Blacklist
func (b *Blacklist) AppendRaw(raw string) bool {
	bl := ParseRules(raw)

	if len(bl.List) == 0 {
		return false
	}

	b.Concatenate(bl.List)
	return true
}

// Concatenate combines a list of Rules to the Blacklist
func (b *Blacklist) Concatenate(items []*Rule) {
	b.List = append(b.List, items...)
}

// ParseRules parses a raw blacklist (text) and returns a Blacklist struct.
// 	Match All [*] (Useful for creating a whitelist)
// 	Match IP [e.g. 203.0.113.6 or 2001:db8::68]
// 	Match IP Network [e.g.: 192.0.2.0/24 or ::1/128]
// 	Match Hostname [e.g. crawl-66-249-66-1.googlebot.com]
// 	Match Hostname RegExp [e.g.: ~ .*\.cox\.net]
// 	Match Geofence [e.g.: @ 39.377297 -74.451082 (7km)] or [ @ Country:IT ] or [ @ City:Rome ]
func ParseRules(rules string) Blacklist {
	lines := strings.Split(rules, "\n")
	blacklist := Blacklist{List: []*Rule{}}

	for _, line := range lines {
		item := &Rule{Raw: line}

		// Ignore blank lines or comments (beginning with #),
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		// An optional prefix "!" which negates the pattern;
		// any matching address/host excluded by a previous pattern
		// will become included again.
		if line[0] == '!' {
			item.Negation = true
			line = strings.TrimSpace(line[1:])
		}

		// A line with only "*" matches everything,
		// allowing the creation of a whitelist.
		if line == "*" {
			item.All = true
			blacklist.Add(item)
			continue
		}

		/*
			// Database query match
			if line[:3] == "db " {
				db := strings.ToLower(strings.TrimSpace(line[3:]))
				if _, ok := dbconfig[db]; !ok {
					item.Comment = fmt.Sprintf("Error: %s: No database specified named %q", line, db)
					blacklist.Add(item)
					continue
				}
				item.Database = db
				blacklist.Add(item)
				continue
			}

		*/

		switch line[0] {
		case '@':
			// An optional prefix "@" indicates a geofencing target.
			// Geofencing can be either with Location coordinates,
			// or by defining values to match, such as Country
			line = strings.TrimSpace(line[1:])

			matches := regexp.MustCompile(`^(\w+):([\w\s]+)$`).FindStringSubmatch(line)
			if len(matches) == 3 {
				item.Geofence = &Geofence{
					Type:  Parameter,
					Field: strings.ToLower(matches[1]),
					Value: strings.ToLower(matches[2]),
				}

				blacklist.Add(item)
				continue
			}

			matches = geofenceRegexp.FindStringSubmatch(line)
			if len(matches) == 5 {
				var lat, lng, radius float64 = 0, 0, 25
				var err error

				latString, lngString, radiusString, units :=
					matches[1], matches[2], matches[3], strings.ToLower(matches[4])

				// Parse latitude
				if lat, err = strconv.ParseFloat(latString, 64); core.IsError(err) {
					// Bad latitude
					continue
				}

				// Parse longitude
				if lng, err = strconv.ParseFloat(lngString, 64); core.IsError(err) {
					// Bad longitude
					continue
				}

				// Parse optional radius
				if radiusString != "" {
					if radius, err = strconv.ParseFloat(radiusString, 64); core.IsError(err) {
						// Bad radius
						continue
					}
				}

				// Parse units
				factor, ok := geofenceUnits[units]
				if !ok {
					// Bad radial units
					continue
				}

				item.Geofence = &Geofence{
					Type:      Location,
					Latitude:  lat,
					Longitude: lng,
					Radius:    radius * factor,
				}

				blacklist.Add(item)
			}

			continue
		case '~':
			// An optional prefix "~" indicates a hostname regular expression match.
			line = strings.TrimSpace(line[1:])
			_, err := regexp.Compile(line)
			if core.IsError(err) {
				blacklist.Add(item)
				continue
			}

			item.Regexp = line
			blacklist.Add(item)
			continue

		case '>':
			// An optional prefix ">" indicates a user-agent match.
			line = strings.TrimSpace(line[1:])
			item.UserAgent = line

			// If > is followed by ~, a regular expression will be applied: e.g. >~ .*curl.*
			if line[0] == '~' {
				line = strings.TrimSpace(line[1:])
				_, err := regexp.Compile(line)
				if core.IsError(err) {
					item.UserAgent = line
					blacklist.Add(item)
					continue
				}

				item.UserAgent = line
				item.Regexp = line
			}

			blacklist.Add(item)
			continue
		}

		// If a CIDR notation is given, then parse that as an IP network.
		_, network, err := net.ParseCIDR(line)
		if err == nil {
			item.Network = network
			blacklist.Add(item)
			continue
		}

		// If an IP address is given, parse as unique IP.
		if ip := net.ParseIP(line); ip != nil {
			item.IP = ip
			blacklist.Add(item)
			continue
		}

		// Otherwise, treat the pattern as a hostname.
		item.Hostname = strings.ToLower(line)
		blacklist.Add(item)
	}

	return blacklist
}

// Allow decides whether the Blacklist permits the selected IP address.
//func (module *Watchdog) Allow(ip net.IP) bool {
func (module *Watchdog) Allow(r *http.Request) bool {

	ip := GetRealAddr(r)
	ua := GetUserAgent(r)

	// TODO: Hardcoded default ALLOW policy, consider to make it customizable.
	allow := true
	b := module.Rules
	var geoCity *geoip2.City

	for _, item := range b.List {
		match := false

		if item.All {
			// Wildcard
			match = true

		} else if item.Network != nil {
			// IP Network
			match = item.Network.Contains(ip)

		} else if item.IP != nil {
			// IP Address
			match = item.IP.Equal(ip)

		} else if item.Hostname != "" {
			// Hostname
			addrs, err := net.LookupIP(item.Hostname)
			if err != nil {
				for _, addr := range addrs {
					if addr.Equal(ip) {
						match = true
						break
					}
				}
			}

			names, err := net.LookupAddr(ip.String())
			if err != nil {
				for _, name := range names {
					name = strings.ToLower(name)
					if name == item.Hostname {
						match = true
						break
					}
				}
			}

		} else if item.Regexp != "" {
			// Regular Expression
			regex, err := regexp.Compile(item.Regexp)
			if core.IsError(err) {
				module.Warning("Error compiling regular expression %s.\n%s", item.Regexp, err)
				continue
			}

			// Regex can apply to:
			// - UserAgent
			// - IP/Network/Etc.

			if item.UserAgent != "" {
				if regex.Match([]byte(ua)) {
					match = true
				}
			} else {
				names, err := net.LookupAddr(ip.String())
				if !core.IsError(err) {
					for _, name := range names {
						name = strings.ToLower(name)
						if regex.Match([]byte(name)) {
							match = true
						}
					}
				}
			}

		} else if item.UserAgent != "" {
			// User-Agent
			match = item.UserAgent == ua

		} else if item.Geofence != nil {

			var err error
			if module.GeoDB == nil {
				continue
			}

			if geoCity == nil {
				geoCity, err = module.GeoDB.City(ip)
				if core.IsError(err) {
					geoCity = nil
					continue
				}
			}

			// Geofence by Parameter
			if item.Geofence.Type == Parameter {
				if item.Geofence.Field == "country" && strings.ToLower(geoCity.Country.IsoCode) == item.Geofence.Value {
					match = true
				} else if item.Geofence.Field == "city" && strings.ToLower(geoCity.City.Names["en"]) == item.Geofence.
					Value {
					match = true
				}
			}

			// Geofence by Location
			if item.Geofence.Type == Location {
				user := &Geofence{
					Latitude:  geoCity.Location.Latitude,
					Longitude: geoCity.Location.Longitude,
					Radius:    float64(geoCity.Location.AccuracyRadius) * 1000.0, // Convert km to m
				}

				bounds := item.Geofence
				boundsIntersect := bounds.Intersection(user)

				if item.Negation {
					// Whitelist if user is completely contained within bounds
					match = boundsIntersect&IsSuperset != 0
				} else {
					// Blacklist if user intersects at all with bounds
					match = !(boundsIntersect&IsDisjoint != 0)
				}
			}

		}

		// TODO: Allow early termination based on negation flags
		if match {
			allow = item.Negation
		}

	}

	if !allow {
		module.Error("Blocked visitor [%s/%s]", tui.Red(ip.String()), tui.Red(ua))
	}

	return allow
}

// MonitorRules starts a watcher to monitor changes to file containing blacklist rules.
func (module *Watchdog) MonitorRules() {

	filepath := module.RulesFilePath

	// starting watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		module.Error(err.Error())
	}
	defer watcher.Close()

	// monitor events
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				switch event.Op {
				case fsnotify.Write:
					module.loadRules()
				}
			case err := <-watcher.Errors:
				module.Error(err.Error())
			}
		}
	}()

	// add file to the watcher first time
	module.Debug("Monitoring %s file changes\n", filepath)
	if err = watcher.Add(filepath); err != nil {
		module.Error(err.Error())
	}

	// to keep waiting forever, to prevent main exit
	// this is to replace the done channel
	select {}
}

// GetRealAddr returns the IP address from an http.Request
func GetRealAddr(r *http.Request) net.IP {

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if parts := strings.Split(forwarded, ","); len(parts) > 0 {
			// Intermediate nodes append, so first is the original client
			return net.ParseIP(strings.TrimSpace(parts[0]))
		}
	}

	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return net.ParseIP(addr)
	}

	return net.ParseIP(r.RemoteAddr)
}

// GetUserAgent returns the User-Agent string from an http.Request
func GetUserAgent(r *http.Request) string {
	return r.UserAgent()
}
