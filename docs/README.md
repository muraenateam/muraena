# Muraena Reverse Proxy

# Intro

Muraena implements a custom Reverse Proxy usingn Golang standard library.

# Requirements

In order to run Muraena in proper way, there is a couple of pre-requisite that
you have to set.

- Generate a wildcard certificate for your phishing domain
- Change or add some settings on the Operating System where Muraena will be run
- Customise your Muraena config file

## Wildcard certificate

Valid wildcard certificate for the domain you want tho use for phishing. A good free option is LetsEncrypt:

```bash
certbot certonly --manual --server https://acme-v02.api.letsencrypt.org/directory --agree-tos -d *.phishing.click -d phishing.click
```

LetsEncrypt will need web and DNS challenges:
* **web challenge**: start apache and create the required file in `/var/www/html` with the required file content

* **DNS challenge**: add the right `TXT` record

## System
 
Muraena needs at least 2x cores and 2x GB RAM. 
If running on AWS, a `t2.medium` (2x vCPU, 2GB ram) is advised for standard campaigns with less than 1K targets.

For the Disk, always use SSD for better performance.

With plenty of simultaneous connections the Reverse Proxy will need mostly CPU and I/O.

If you plan to run large campaigns, in the order of more than 2K people emailed at the same time, expecting hundreds of simultaneous clicks, then a `c5.xlarge` (4x vCPU, 8GB ram) instance will be better.

The VPS performance depends also on the complexity of the site being reverse proxied.
Heavy sites like Atlassian portals or GSuite have more traffic to be handled than a bogus login portal on PHP ;-)


### `Ulimit` increase
 
If a lot of victims connect at the same time, the default open files settings are not enough. It is recommended to increase to max the following:

```bash
$ sudo vim /etc/sysctl.conf
# add the following line to it
fs.file-max = 65535

$ sudo vim /etc/security/limits.conf
# add following lines to it
* soft     nproc          65535    
* hard     nproc          65535   
* soft     nofile         65535   
* hard     nofile         65535
root soft     nproc          65535    
root hard     nproc          65535   
root soft     nofile         65535   
root hard     nofile         65535

$ sudo vim /etc/pam.d/common-session
# add this line to it
session required pam_limits.so
```

Reboot the machine. 

> **NOTE**: for the Muraena Reverse Proxy is good to assign an Elastic IP to it, so it never changes, and less DNS changes are needed, so the domain looks less suspicious.

Test with `ulimit -n` if you get > 65K files.

### Redis installation

Muraena use Redis as a database.

Nothing specific is required for Redis, just follow instruction from
[Redis](https://redis.io/topics/quickstart) for the installation.

```bash
$ sudo apt-get install redis-server
$ sudo systemctl enable redis-server.service
$ sudo vim /etc/redis/redis.conf
maxmemory 256mb
maxmemory-policy allkeys-lru
$ sudo systemctl restart redis-server.service
```

Verify Redis works

```bash
$ redis-cli ping
PONG
```

> **NOTE**: Don't change the TCP port and keep port **6379**.

## Proxy TOML configuration

Main things to be changed are:

* proxy.phishing = yourphishingdomain.com
* proxy.destination = therealdomain.com
* tls.key/certificate = need to be updated with the LetsEncrypt data
* drop.path/redirectTo = prevent logout or redirect
* tracking.enabled = true
* tracking.identified = Victim UUID param name choosen when creating a Victim Group in Muraena Portal
* tracking.urls/patterns = needs to be updated depending on what you want to harvest

If you want to use also NecroBrowser, you have set:

* necrobrowser.enabled = true
* necrobrowser.endpoint = the host where NecroBrowser is runninng
* necrobrowser.profile = config to forward to NecroBrowser 
 
#  How to run Muraena Proxy

At this point the proxy can be started, and the victim can land on the phishing lures:

It's recommended to run muraena in a terminal multiplexer like GNU Screen or
Tmux.

```bash
$ screen -S muraena 
$ sudo ./muraena -config config.toml
```

or

```bash
$ tmux -S muraena 
$ sudo ./muraena -config config.toml
```

Harvested credentials/data and authenticated sessions will be logged to STDOUT
but also in Redis.

If NecroBrowser is enabled, authenticated session will be passed with the right profile to be instrumented with NecroBrowser.

# How to debug Muraena Proxy

If you are running the proxy for the first time on a new target, enable the crawler in a clean basic config.toml (from the public Muraena GitHub repo), and see how the target is crawled. 

For complex targets, it happens that the crawler will not be able to automatically identify all the FQDNs that need to be translated during proxying.
This will result in `crawler.externalOrigins` missing some entries, and errors thrown in the browser when you try to proxy.

The best thing to fix this is to compare the `externalOrigins` with the list from Burp Proxy when you proxy the target domain you are trying to proxy with Muraena.

> **NOTE** `externalOrigins` entries support wildcard, so if you see plenty of `a.target.com`, `b.target.com`, etc. you can just use a single entry as **`*.target.com`**.

Another important point for advanced usage is using Muraena to patch requests or responses, removing JavaScript checks or additional bespoke controls that prevent the proxy from working.
There are example of these for GSuite and other portals in the config files in the public Muraena Proxy repo on GitHub.

------------

