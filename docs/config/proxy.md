---
title: Proxy
layout: default
permalink: /config/proxy
nav_order: 1
parent: Configuring Muraena
---

# Proxy

The proxy configuration controls how Muraena handles traffic routing between the phishing target and the 
legitimate destination.

## Settings

### Phishing
The phishing domain you're proxying traffic from, i.e., the domain you're using to lure victims.

### Destination
The legitimate domain you're proxying traffic to, i.e., the domain you're impersonating.

### IP
The IP address Muraena listens on. Defaults to all interfaces (`0.0.0.0`).

### Listener
You could specify the network listener type. The supported listener types are:
- `tcp`
- `tcp4`
- `tcp6`

### Port
The port Muraena listens on, when not specified, it defaults to:
- `80` for HTTP
- `443` for HTTPS, when TLS is enabled

### Port Mapping
If Muraena is running behind a reverse proxy, you can specify the port mapping using the `portMapping` setting.
This is useful when Muraena is running behind a reverse proxy that forwards traffic to a different port.

The mapping format is `source:destination`, where `source` is the port Muraena listens on, 
and `destination` is the port the reverse proxy forwards traffic to.

> For example, if Muraena listens on non-standard port `55443`, but the target domain is configured to listen on port `443`,
you can specify `55443:443` to map the traffic to the correct port.
> 
> Respectively, if Muraena listens on port `443`, but the target domain is configured to listen on non-standard port 
> `55443`, you can specify `443:55443` to map the traffic to the correct port.


### HTTP to HTTPS Redirect
When Muraena is configured to listen on HTTPS, HTTP traffic won't be handled by default.

Enabling `HTTPtoHTTPS`, upon receiving an HTTP request, Muraena will redirect the request to the HTTPS equivalent URL, 
by replacing the `http` scheme with `https`, patching the port if necessary and returning a `301 Moved Permanently` 
status code.

#### Parameters
- **`enabled`**: (default `false`) Enable or disable the HTTP to HTTPS redirect
- **`port`**: (default `80`) The port to listen for HTTP traffic before redirecting to HTTPS


## Examples

### Basic Example

This example sets up Muraena to listen on port 80 for HTTP traffic, redirecting to HTTPS,
and to proxy traffic from `phishing.click` to `poor.victim`.
All other settings are left to their default values.

```toml
[proxy]
phishing = "phishing.click"
destination = "poor.victim"
```


### Advanced Example

The following example sets up Muraena to listen on IP `192.168.1.1` only in IPv4 mode.
It listens on port `55443` for HTTPS traffic and on port `55080` for HTTP traffic.
However, it's permanently redirecting all HTTP traffic to HTTPS.
The phishing domain is `phishing.click`, and the legitimate domain is `poor.victim`.

```toml
[proxy]
phishing = "phishing.click"
destination = "poor.victim"

IP = "192.168.1.1"
listener = "tcp4"
port = 55443
portmapping = "55443:443"

[proxy.HTTPtoHTTPS]
enabled = true
port = 55080
```