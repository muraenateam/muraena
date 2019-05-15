<p align="center">
  <img alt="Muraena Logo" src="./media/img/muraena-logo.jpg"
   height="160" /><br>
	<i>help us with a logo</i>
</p>

**Muraena** is an almost-transparent reverse proxy aimed at automating phishing and post-phishing activities.

About
-------------

The tool re-implements the 15-years old idea of using a custom reverse proxy to dynamically interact with the 
origin to be targeted, rather than maintaining and serving static pages.

Written in Go, Muraena does not use slow-regexes to do replacement magic, and embeds a crawler (Colly)
that helps determining in advance which resource should be proxied.

Muraena does the bare minimum to grep/replace origins in request/responses: this means
that for complex origins extra manual analysis might be required to tune the auto-generated JSON configuration file.
Hence, do not expect the reverse proxy to work straight out of the box for complex origins. 

The config folder has some examples of custom replacements needed on complex origins likes GSuite, Dropbox, GitHub 
and others.


Testing Setup
-------------

With `dnsmasq` handling a new TLD that we use just for testing, for example .anti, you have the following
in `/usr/local/etc/dnsmasq.conf`:

`address=/.anti/127.0.0.1`

Once verified that you can resolve `anything.goes.to.anti`, you need a wildcard certificate for your phishing domain.


For testing purposes it's more than enough this awesome tool: [mkcert](https://github.com/FiloSottile/mkcert).
_"A simple zero-config tool to make locally trusted development certificates with any names you'd like"_


Once certificates are sorted, just include them within the configuration file:

```json
"tls": {
    "enabled": true,
    "expand": false, 
    "certificate": "./config/phishing-cert.pem",
    "key": "./config/phishing-key.pem",
    "root": "./config/phishing-rootCA.pem",
}
```

or Base64 encode the certificates: 

```
alias cert2base64='awk '\''{printf "%s\\n", $0}'\'' '
cert2base64 <certificate.pem> | pbcopy
```
and paste in their JSON fields:
```json
"tls": {
    "enabled": true,
    "expand": false, 
    "certificate": "-----BEGIN CERTIFICATE-----[...]]", 
    "key": "-----BEGIN RSA PRIVATE KEY-----[...]",
    "root": "-----BEGIN CERTIFICATE-----[...]"
}
```

Public Setup
-------------
In real life you need a certificate from a public CA unless somehow your targets already trust your custom CA.
An free option is to use LetsEncrypt. Once you obtained your wildcard certificate, just point the key and certificate material to the config file in the same way as described in the [Testing Setup](#testing-setup).

Similarly, `dnsmasq` is not an option, so you will need to tune the DNS Zone file of your phishing domain
(which you partially already did to get the LetsEncrypt, see `A` record) in order to have
a wildcard `CNAME` like the following:

```
* 10800 IN CNAME phishing.anti.
```


Running the beast
-----------------
Muraena comes with a Makefile, so just:

`make build`

After you go build, make sure the certs are there, set the 'destination' in
the default config file in config/config.json, then just start it:

`sudo ./muraena`

The target destination is crawled, and a new config file is created based on the name of the 'destination'
configuration option. You can already proxy through Muraena at this stage,
but it is recommended that you double-check the generated configuration to see
if the crawler did not miss any important origins which are external to your main proxied domain.
Once you are satisfied with the generated config file, start with:

`sudo ./muraena -c your_config.json`


Compiling from Sources
-----------------

In order to compile muraena from sources, make sure that:

* You have a correctly configured **[Go >= 1.12](https://golang.org/doc/install)** environment.
* `$GOPATH` is defined and `$GOPATH/bin` is in `$PATH`.

Once you've met this conditions, you can run the following commands to compile muraena:

```bash
go get github.com/muraenateam/muraena
cd $GOPATH/src/github.com/muraenateam/muraena
make build 
```

MiTM the Proxy
-----------------

Extra support for debugging the proxy. If `HTTP_PROXY` or `HTTPS_PROXY` env variables are defined all the proxy 
traffic will be forwarded to the defined proxy if `-debug` switch is set. 

```
sudo -E http_proxy=127.0.0.1:8080 ./muraena -debug true -config yourconfig.json
```
		
Notes
-----------

Great Go libraries that did help during development:
  - reverse proxy: Go core net/httputil
  - crawler: https://github.com/gocolly/colly
  - browser instrumentation: https://github.com/chromedp/chromedp
  - extract URLs from strings: https://github.com/mvdan/xurls


Credits
-----------

- @drk1wi for being an ex muraena-team member and for his great contribution
- @evilsocket for being a source of inspiration
- @kgretzky for giving us a motivation to make a better solution :p


## License

`muraena` is made with ❤️ by [the dev team](https://github.com/orgs/muraenateam/people) and it's released under the 3-Clause BSD License.
