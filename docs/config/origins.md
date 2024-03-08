---
title: Origins
permalink: /docs/origin
nav_order: 2
parent: Configuring Muraena
---

# Origins

During a phishing operation, Muraena can impersonate multiple domains, and it can proxy traffic to multiple legitimate domains.

Muraena maps the phishing domain to the legitimate domain, and it can also map subdomains between the phishing site and 
the legitimate site. For example, if the phishing domain is `phishing.click` and the legitimate domain is `poor.victim`, 
Muraena will map the phishing domain to the legitimate domain.
Additionally, all subdomains of `phishing.click` will be mapped to the corresponding subdomains of `poor.victim`, 
ensuring that the phishing site can mimic the legitimate site as closely as possible.

This means that the following mappings will be created automatically:
- `www.phishing.click`   -> `www.poor.victim`
- `admin.phishing.click` -> `admin.poor.victim`
- `api.phishing.click`   -> `api.poor.victim`
- ...

In addition to the legitimate domain, Muraena can also proxy traffic to other external origins, such as third-party 
services, APIs, or other legitimate domains. This is useful when the phishing site needs to interact with external 
services, such as fetching resources from a CDN or submitting data to a third-party service.

Each external origin is internally numbered and mapped to a subdomain of the phishing domain, allowing the phishing site 
to interact with the external origin as if it were the legitimate site.
The subdomain prefix is defined in the `ExternalOriginPrefix` setting, and the external origins are defined in the 
`ExternalOrigins` setting.

For example, if the `ExternalOriginPrefix` is set to `ext`, and the `ExternalOrigins` to map are:
`api.external.com`, `cdn.external.com` and `cdn.anotherexternal.com`, Muraena will map the phishing domain to the 
external origins as follows:

- `ext-1.phishing.click` -> `api.external.com`
- `ext-2.phishing.click` -> `cdn.external.com`
- `ext-3.phishing.click` -> `cdn.anotherexternal.com`


Muraena can also handle wildcard external origins, so you can use `*.external.com` to match all subdomains of `external.com`.

In addition to the origins, Muraena can also map subdomains between the phishing site and the target site.
This is useful when the phishing site wants to further mimic the legitimate site by using the different subdomains.
This can be achieved using the `SubdomainMap` setting.


## Settings

### External Origin Prefix
The `externalOriginPrefix` setting defines the prefix used to identify the external origins, i.e., 
the legitimate domains you're proxying traffic to. 
The prefix must be a valid subdomain name, without any dot, and must respect the following regex pattern: 
`^[a-zA-Z0-9-]+$`.


### External Origins
The `externalOrigins` setting is a list of legitimate domains you're proxying traffic to, in addition to the legitimate domain 
you're impersonating. The domains are specified as a list of strings, and each domain is mapped to a subdomain of the 
phishing domain, using the `externalOriginPrefix` as a prefix.
Domains can be also specified as wildcard domains, using `*` as a prefix, to match all subdomains of the domain.

> **NOTE:** There is no need to specify subdomains of the target domain, the one specified in the `proxy.Destination` 
> setting, as Muraena will automatically map all subdomains of the phishing domain to the corresponding subdomains of 
> the target domain.

#### Example
```toml
[origins]
externalOriginPrefix = "ext"
externalOrigins = [
    "*.external.com",
    "cdn.anotherexternal.com"
]
```


### Subdomain Map
The `subdomainMap` is a list of subdomain pairs, where the first element is the phishing subdomain, 
and the second element is the legitimate subdomain.
`subdomainMap` allows custom mapping of subdomains between the phishing site and the legitimate site.
This is useful when the phishing site wants to further mimic the legitimate site by using the different subdomains.


```toml
[origins]
subdomainMap = [
    # phishing subdomain -> legitimate subdomain
    ["www", "admin"]
]   
```

> **NOTE:** This mapping applies only to the subdomains of the target domain, not to other external origins


## Examples

```toml
[origins]

externalOriginPrefix = "ext"

externalOrigins = [
    "*.external.com",
    "cdn.anotherexternal.com"
]

subdomainMap = [
    ["www", "www2"]
]   
```

