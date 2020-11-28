<p align="center">
  <img alt="Muraena Logo" src="./media/img/muraena-logo.jpg"
   height="160" /><br>
	<i>help us with a logo</i>
	<p align="center">
    <a href="https://github.com/muraenateam/muraena/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/muraenateam/muraena.svg?style=flat-square"></a>
    <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/muraenateam/muraena"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/muraenateam/muraena?style=flat-square&fuckgithubcache=1"></a>
  </p>
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


## Contributing

1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request 🤩

See the list of [contributors](https://github.com/muraenateam/muraena/contributors) who participated in this project.

## License

**Muraena** is made with ❤️ by [the dev team](https://github.com/orgs/muraenateam/people) and it's released under the <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>.
