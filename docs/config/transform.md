---
title: Transform
layout: default
permalink: /docs/transform
nav_order: 3
parent: Configuring Muraena
---

# Transform

In phishing operations, effectively altering the HTTP traffic between the victim and the legitimate site is essential 
for maintaining the authenticity of the phishing site. 
The Transform section in Muraena's configuration provides the necessary settings for detailed manipulation of HTTP 
requests and responses. 

This capability ensures that the phishing site not only mirrors the appearance of the genuine site but also replicates 
its behavior, enhancing the credibility of the phishing campaign.

This section will guide you through the configuration of transformation rules, focusing on the technical aspects of how 
to intercept and modify traffic. You'll learn how to encode content, manage MIME types, customize user agents, 
map subdomains, and transform headers and content, all of which are pivotal in crafting a convincing phishing site.


## Settings

### `base64`

By enabling `base64` Muraena will try to transform any content, both request and response, that is Base64 encoded.
This is useful when the target site uses Base64 encoding for specific data elements, such as tokens or cookies,
and you want to ensure that the phishing site can handle these elements correctly.

#### Parameters
- **`enabled`** (default `false`): Toggles Base64 encoding for parts of the communication.
- **`padding`** (default `["=", "."]`): Specifies the padding characters used in Base64 encoding, which can be adjusted 
to match the encoding specifications of the target site.

### Request 
The Request section specifies where the transformation rules should be applied to the requests sent from the phishing 
server to the legitimate site. 

#### `userAgent`
You can specify a custom User-Agent string to be used in the requests sent from the phishing server to the legitimate site.

#### `headers`

`headers` defines a list of HTTP headers to be transformed during the request phase. 
HTTP headers usually contain metadata about the request, and modifying them can help in bypassing certain security
controls as well as avoid leaking information about the phishing server.
For example, `Referer` headers can be modified to ensure that the phishing site's URL is not leaked to the legitimate site.

Commonly headers to transform include:
- `Cookie`
- `Referer`
- `Origin`
- `X-Forwarded-For`


#### `remove`
##### `headers`
`headers` defines a list of HTTP headers to be removed during the request phase.
HTTP headers usually contain metadata about the request, and removing them can help in bypassing certain security
controls as well as avoid leaking information about the phishing server.

For example, if Muraena is running behind a reverse proxy, you might want to remove the `X-Forwarded-For` header to avoid
leaking the real client's IP address to the legitimate site.
Or if you're using a custom header to track requests, you might want to remove it to avoid leaking information about the
phishing server.

Commonly headers to transform include:
- `X-Forwarded-For`


#### `add`
##### `headers`
`headers` defines a list of pairs of HTTP headers to be added during the request phase.
The first element is the header name and the second element is the header value.

For example, you might want to add a custom header to track requests, or to add a header to bypass security controls on
the legitimate site.

```toml
[transform.request]
add.headers = [
    {name = "X-Phishing-Header", value = "Phishing"}
]
```

### Response 
The Response section specifies where the transformation rules should be applied to the responses sent from the legitimate 
site to the phishing server.

Transforming the response from the legitimate site is key to maintaining the phishing site's facade. 
This includes modifying both HTTP headers and body to ensure they point back to the phishing domain.


#### `skipContentType`

Muraena will try to transform any response content. However, certain content-types might not need transformation,
either for performance considerations or to maintain functionality (like binary data or certain scripts), see for
example the `font/*` and `image/*` content types.
By specifying `skipContentType`, you can define a list of MIME types that should not be transformed or encoded,
ensuring proper handling of non-text content.

The `skipContentType` is a list of MIME types that should not be transformed or encoded, ensuring proper handling of 
non-text content. You could use wildcards to match multiple content types, for example, `image/*` would match all image 
types, and `font/*` would match all font types.

If `skipContentType` is not specified, Muraena will skip transformation for the following content types:
- `font/*`
- `image/*`

##### Example
The following example skips transformation for `image/jpeg` and all font types.

```toml
[transform]
skipContentType = ["image/jpeg", "font/*"]
```


#### `headers`

`headers` defines a list of HTTP headers to be transformed during the response phase.
HTTP headers usually contain metadata about the response, and modifying them can help in bypassing certain security
controls as well as avoid leaking information about the legitimate site.
For example, `Location` headers can be modified to ensure that the real site's URL is changed to the phishing site's URL.

Commonly headers to transform include:
- `Location`
- `WWW-Authenticate`
- `Origin`
- `Set-Cookie`
- `Access-Control-Allow-Origin`


#### `customContent`
`customContent` defines a list of content transformation rules to be applied to both response headers and body.

The rules are defined as a list of pairs, where the first element is the search string and the second element is the 
replacement string. `customContent` works by searching for the `search` string in the response content and replacing it 
with the `replace` string.

##### Example
This rule modifies all occurrences of `integrity=` to `integrify=` within the response content. 
Such a modification aids in circumventing the `integrity` attribute found within `<script>` tags, which serves to verify 
the script content's integrity. 
By substituting `integrity` with an alternate attribute, namely `integrify`, the phishing site is enabled to execute 
altered scripts unimpeded by integrity verification mechanisms. The browser overlooks the script content's integrity 
check in this scenario, as it perceives `integrify` as an unrelated attribute and consequently disregards it.

```toml
[transform.response]

customContent = [
    # search    ->    replace
    ["integrity=", "integrify="]
]
```

#### `cookie`

`cookie` defines a list of cookie transformation rules to be applied to the response cookies.

##### Parameters

- **`sameSite`**: Sets the cookie's `SameSite` attribute to `None`, `Lax`, or `Strict`. 
  If not specified, it is left unchanged.



#### `remove`
##### `headers`
`headers` defines a list of HTTP headers to be removed during the response phase.
HTTP headers usually contain metadata about the response, and remove them can help in bypassing certain security
controls as well as avoid leaking information about the legitimate site.
Removing headers can also help weaken security controls on the legitimate site, such as removing `Content-Security-Policy`
headers to allow for more flexible content injection.

Commonly headers to transform include:
- `Content-Security-Policy`
- `Content-Security-Policy-Report-Only`
- `Report-To`
- `X-Content-Type-Options`
- `X-Frame-Options`
- `Referrer-Policy`

#### `add`
##### `headers`
`headers` defines a list of pairs of HTTP headers to be added during the response phase.
The first element is the header name and the second element is the header value.

For example, you might want to add a custom header to track responses, or to add a header to bypass security controls on
the legitimate site.

```toml
[transform.request]
add.headers = [
    {name = "X-Phishing-Header", value = "Phishing"}
]
```


## Examples

### Basic Transform Example

```toml
[transform]

[transform.request]
headers = [
  "Cookie", 
  "Referer", 
  "Origin", 
  "X-Forwarded-For"
]

[transform.response]
headers = [
  "Location",
  "Origin",
  "Set-Cookie",
  "Access-Control-Allow-Origin",
]
```

### Advanced Transform Example


```toml
[transform]

[transform.base64]
enabled = true

[transform.request]
userAgent = "Mozilla/5.0 (PhishingBot)"

headers = [
"Cookie",
"Referer",
"Origin",
"X-Forwarded-For"
]

remove.headers = [
  "X-Forwarded-For"
]

add.headers = [
  {name = "X-Phishing-Header", value = "Phishing"}
]

[transform.response]
skipContentType = ["image/jpeg", "font/*", "application/*"]

headers = [
"Location",
"Origin",
"Set-Cookie",
"Access-Control-Allow-Origin",
]

customContent = [
["integrity=", "integrify="]
]

remove = [
  "Content-Security-Policy",
  "Content-Security-Policy-Report-Only",
  "Report-To",
  "X-Content-Type-Options",
  "X-Frame-Options",
  "Referrer-Policy"
]

add = [
  {name = "X-Phishing-Header", value = "Phishing"}
]

```
