---
title: TLS
layout: default
permalink: /docs/tls
nav_order: 5
parent: Configuring Muraena
---

# TLS

This section guides you through configuring TLS for your Muraena setup,
detailing how to enable HTTPS, manage certificate paths, and adjust SSL/TLS parameters. 
By meticulously configuring these settings, you create a secure and authentic-looking façade 
that effectively masks the malicious nature of the phishing server, 
thereby increasing the likelihood of successful credential capture.

## Settings

### `Enabled`
When enabled, Muraena listens for incoming connections over HTTPS.

### <s>`Expand`</s>
> **NOTE**: This is a deprecated option and will be removed in future versions.

When enabled, Muraena will expand store the certificates content directly in the configuration file.


### `Certificate`
Path to the TLS certificate file. 

### `Key`
Path to the TLS private key file. 

### `Root`
Path to the root CA certificate file, if needed for chain verification.

### `SSLKeyLog`
If set, Muraena will log the SSL keys to the specified file, which can be useful for debugging encrypted traffic.
This option is particularly useful when you need to decrypt SSL/TLS traffic using Wireshark or similar tools.

You could use [tshark](https://www.wireshark.org/docs/man-pages/tshark.html) to dump all the incoming traffic to a file 
and then use Wireshark to decrypt the traffic using the SSL keys log file.

```bash
# Dump all the incoming traffic on:
# -i any: listen on all interfaces, you should replace this with the interface used by Muraena
# -f "port 443": filter traffic on port 443, you should replace this with the port listened by Muraena
# -w muraena_$(date +%y_%m_%d_%H_%M_%S).pcapng: write the traffic to a file
# -v: verbose mode
tshark -i any -f "port 443" -w muraena_$(date +%y_%m_%d_%H_%M_%S).pcapng -v
```


// Minimum supported TLS version: SSL3, TLS1, TLS1.1, TLS1.2, TLS1.3
MinVersion               string `toml:"minVersion"`
MaxVersion               string `toml:"maxVersion"`
PreferServerCipherSuites bool   `toml:"preferServerCipherSuites"`
SessionTicketsDisabled   bool   `toml:"SessionTicketsDisabled"`
InsecureSkipVerify       bool   `toml:"insecureSkipVerify"`
RenegotiationSupport     string `toml:"renegotiationSupport"`



### `MinVersion`
The minimum supported TLS version. Supported values are:
- `SSL3`
- `TLS1` (default)
- `TLS1.1`
- `TLS1.2`
- `TLS1.3`

### `MaxVersion`
The maximum supported TLS version. Supported values are:
- `SSL3`
- `TLS1`
- `TLS1.1`
- `TLS1.2`
- `TLS1.3` (default)


### `RenegotiationSupport`
Defines the TLS renegotiation support mode. Supported values are:
- `NEVER` (default): Disables renegotiation.
- `ONCE`: Allows renegotiation once per connection.
- `FREELY`: Allows renegotiation at any time.

The renegotiation options might be useful in specific scenarios, 
such as when you need to support legacy clients or servers that require renegotiation support.


### <s>`PreferServerCipherSuites`</s>
> **NOTE:** PreferServerCipherSuites is a legacy field and has no effect.

It used to control whether the server would follow the client's or the
server's preference. Servers now select the best mutually supported
cipher suite based on logic that takes into account inferred client
hardware, server hardware, and security.


### `SessionTicketsDisabled`
`SessionTicketsDisabled` may be set to `true` to disable session ticket and
PSK (resumption) support. Note that on clients, session ticket support is
also disabled if ClientSessionCache is nil.


### `InsecureSkipVerify`    
InsecureSkipVerify defines whether Muraena verifies the server's certificate chain and host name.
`InsecureSkipVerify` in Muraena is set to `false` by default, which means that the server certificate verification is enabled.
However, you can set it to `true` to skip the server certificate verification, 
in the case of self-signed certificates or other scenarios where the target server's certificate cannot be verified.


## Examples

### Basic Example

This example enables Muraena to listen for incoming connections over HTTPS, using the specified certificate and key files.
All other settings are left to their default values.

```toml
[tls]
enabled = true
certificate = "./config/cert.pem"
key = "./config/key.pem"
root = "./rootCA.pem"
```

### Advanced Example

The following example enables TLS and sets the minimum and maximum TLS versions to `TLS1.2` and `TLS1.3` 
respectively. It also disables the server certificate verification and logs the SSL keys to `./log/sslkey.log`.

```toml
[tls]
enabled = true
certificate = "./config/cert.pem"
key = "./config/key.pem"
root = "./config/rootCA.pem"
sslKeyLog = "./log/sslkey.log"

minVersion = "TLS1.2"
maxVersion = "TLS1.3"
insecureSkipVerify = true
```