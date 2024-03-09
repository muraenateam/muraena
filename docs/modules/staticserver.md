---
title: Static Server
layout: default
permalink: /modules/staticserver
nav_order: 3
parent: Supported Modules
---

# Static Server

Muraena incorporates the capability to host and serve static files, such as custom JavaScript, CSS, images, or
downloadable content, directly from a designated local directory.
This feature is particularly useful for enriching the phishing site with additional resources that enhance its
resemblance to the legitimate target site or for distributing files intended for the victim.

The Static Server functionality is straightforward: it establishes a direct mapping between a specific URL path on the
phishing site and a folder on the local file system. When a request is made to this URL path,
Muraena responds by serving the corresponding file from the mapped local directory, seamlessly integrating it into
the phishing site's content.

## Configuration Options

### Local Path
Defines the file system path (`localPath`) from which static files will be served. This path should contain the static
resources you wish to make available through the phishing site.

### URL Path
Specifies the URL path (`urlPath`) that will be used to access the static files from the phishing site.
Setting this to `/static`, for example, would make the static files accessible via `http(s)://<phishing.site>/static/`.

### Listening Host
Determines the network interface (`listeningHost`) Muraena listens on for requests to the Static Server.
By default, it listens on all available interfaces, but it can be set to a specific IP address if needed.

Considering that the static server is "fronted" behind Muraena, this setting could be ignored, unless
you have a specific requirement to bind the static server to a specific IP address, maybe for a multi-homed server.

### Listening Port
Defines the port (`listeningPort`) on which the Static Server will listen for incoming requests. While this can be set
to a specific port, leaving it unspecified allows Muraena to select a port at random, which might be useful for
avoiding conflicts with other local services.

Similarly to the `listeningHost` setting, this setting could be ignored, unless you have a specific requirement to
bind the static server to a specific port.

## Example

The following example demonstrates how to configure the Static Server to serve files from the `/var/www/static`
directory on the phishing site under the `/static` URL path:

```toml
[staticServer]
enable = true
localPath = "/var/www/static"
urlPath = "/static"
```
