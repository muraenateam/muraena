---
title: Redirect
layout: default
permalink: /docs/redirect
nav_order: 4
parent: Configuring Muraena
---

# Redirect

The `redirect` section of the configuration file allows you to specify rules to redirect unwanted traffic to a different URL.

When a rule matches the request, the request will be redirected to the specified URL with the specified HTTP status code 
without reaching the legitimate site.

`Hostname`, `Path`, and `Query` are used to match the request, while `RedirectTo` and `HTTPCode` are used to specify 
the redirection.


## Settings

### Hostname

The `hostname` field specifies the hostname to match against the request.

### Path

The `path` field specifies the path to match against the request.

### Query

The `query` field specifies the query string to match against the request. 
The query string is the part of the URL that comes after the `?` character, and it contains a list of key-value pairs 
separated by `&` characters. 

### Redirect to

The `redirectTo` field specifies the URL to redirect the request to. If the request matches the specified values, it will be redirected to the specified URL.

### HTTP Status Code

The `httpStatusCode` field specifies the HTTP status code to use for the redirection. If not specified, it defaults to `301 Moved Permanently`.


## Example
```toml

[drop]
    [[drop]]
        hostname = "phishing.click"
        path = "/login"
        query = "id=123"
        redirectTo = "https://poor.victim/login"
        httpStatusCode = 301
    
    [[drop]]
        hostname = "phishing.click"
        path = "/admin"
        redirectTo = "https://poor.victim/admin"
        httpStatusCode = 301
    
    [[drop]]
        hostname = "analytics.local"
        redirectTo = "https://poor.victim"
        httpStatusCode = 301
```