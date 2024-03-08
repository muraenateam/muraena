---
title: Tracking Configuration
layout: default
permalink: /modules/tracker
nav_order: 1
parent: Supported Modules
---

# Tracking Configuration

The Tracking module in Muraena is an essential tool for monitoring user interactions and capturing sensitive information
during a phishing campaign. It provides a detailed framework for tracking user activities, from initial landing to
sensitive data capture, enhancing the operational effectiveness of the campaign.

## Settings Overview

### Enable
Enables or disables the entire tracking functionality. When `enable` is set to `true`, tracking features are activated,
allowing for the monitoring of user interactions and data capture.

### Track RequestCookies

`trackRequestCookies` flag is used to enable or disable the tracking of cookies in user requests.
When enabled, this feature allows Muraena to keep track of cookies in user requests.
This is useful for tracking client-side state and user sessions that are maintained through cookies.


### Trace
This section is dedicated to tracing user navigation within the phishing site, allowing for the identification and
redirection of users based on specific criteria.

- **`identifier`**: A unique identifier for tracking purposes, this string is used to track requests and identify users.
- **`header`**: Specifies an HTTP header used as part of the tracking mechanism, enabling the capture of custom header
  values. (Default: `If-Range`)
- **`domain`** (optional): Tells Muraena to create tracking cookies for the specified domain. This is required if you want
  to specify a domain different from the phishing site's domain.
- **`validator`**: A regular expression used to validate the victim's identifier. (Default: it must be a valid UUIDv4)

#### Landing
Configures how Muraena identifies and handles user landings on the phishing site.

- **`type`**: Determines the method of landing detection (`path` or `query`), allowing for flexibility in how landing
  pages are recognized.
- **`header`**: An HTTP header that signals a landing event, useful for tracking landings through header analysis.
  (Default: `If-LandingHeader-Redirect`)
- **`redirectTo`**: Specifies a URL to redirect users to after a landing is detected. This setting is applicable only
  when the landing type is set to `path`.

### Secrets
Focuses on capturing sensitive information, such as credentials or personal data, through specified paths and pattern
matching.

#### Paths
`paths` is defines the list of URL paths monitored for sensitive information.
Paths can be specified as regular expressions to match multiple paths, or as exact paths to match a single path.
In order to consider a path as a regular expression, it must start with `^` and end with `$`.

For example, to match all paths that start with `/login` you can use the following regular expression: `^/login.*$`.

#### `Patterns`
Defines specific patterns for data capture, enhancing the precision of sensitive information extraction.

- **`label`**: A descriptive name for the pattern, aiding in the identification and categorization of captured data.
- **`matching`** (optional): The string used to identify sensitive information within the monitored traffic.
- **`start`** and **`end`**: Once the `matching` string is found, `start` and `end` are used to define the bounds of the
  data to be extracted, ensuring accurate and efficient data capture.


## Examples

Below is an example configuration demonstrating the setup for user tracing and sensitive data capture:

```toml
[tracking]
enable = true
trackRequestCookies = true

[tracking.trace]
identifier = "user_id"
header = "X-Tracking-ID"
validator = "[a-zA-Z0-9]{5}"

[tracking.trace.landing]
type = "path"
header = "Landing-Detected"
redirectTo = "https://phishing.site/welcome"

[tracking.secrets]
paths = ["/login", "/submit"]

[[tracking.secrets.patterns]]
label = "Credential Capture - Username"
start = "username="
end = "&"
```