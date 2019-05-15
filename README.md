<p align="center">
  <img alt="Muraena Logo" src="./media/img/muraena-logo.jpg"
   height="160" /><br>
	<i>help us with a logo</i>
</p>

**Muraena** is an almost-transparent reverse proxy aimed at automating phishing and post-phishing activities.

The tool re-implements the 15-years old idea of using a custom reverse proxy to dynamically interact with the 
origin to be targeted, rather than maintaining and serving static pages.

Written in Go, Muraena does not use slow-regexes to do replacement magic, and embeds a crawler (Colly)
that helps determining in advance which resource should be proxied.

Muraena does the bare minimum to grep/replace origins in request/responses: this means
that for complex origins extra manual analysis might be required to tune the auto-generated JSON configuration file.
Hence, do not expect the reverse proxy to work straight out of the box for complex origins. 

The config folder has some examples of custom replacements needed on complex origins likes GSuite, Dropbox, GitHub 
and others.


## Documentation

The project is documented in the [Wiki here](https://github.com/muraenateam/muraena/wiki).


## Credits

Great Go libraries that did help during development:
  - reverse proxy: Go core net/httputil
  - crawler: https://github.com/gocolly/colly
  - browser instrumentation: https://github.com/chromedp/chromedp
  - extract URLs from strings: https://github.com/mvdan/xurls

- @drk1wi for being an ex muraena-team member and for his great contribution
- @evilsocket for being a source of inspiration
- @kgretzky for giving us a motivation to make a better solution :p


## License

`muraena` is made with ❤️ by [the dev team](https://github.com/orgs/muraenateam/people) and it's released under the 3-Clause BSD License.
