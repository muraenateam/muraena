---
layout: default
title: Running Muraena
permalink: /infra/run
nav_order: 1
has_toc: true
---

# Running Muraena

## Requirements

In order to run Muraena in proper way, there is a couple of pre-requisite that
you have to set.

- Generate a wildcard certificate for your phishing domain
- Change or add some settings on the Operating System where Muraena will be run
- Customise your Muraena config file

**Wildcard certificate**

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

If you plan to run large campaigns, in the order of more than 2K people emailed at the same time, expecting hundreds of 
simultaneous clicks, then a `c5.xlarge` (4x vCPU, 8GB ram) instance will be better.

The VPS performance depends also on the complexity of the site being reverse proxied.

### `Ulimit` increase
 
If a lot of victims connect at the same time, the default open files settings are not enough. 
It is recommended to increase to max the following:

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

# Reboot the machine!
```

Test with `ulimit -n` if you get > 65K files.

### Redis installation

Muraena uses Redis as a database to store the harvested credentials and sessions.
Nothing specific is required for Redis, just follow instruction from [Redis](https://redis.io/topics/quickstart) for the installation.

```bash
sudo apt-get install redis-server
sudo systemctl enable redis-server.service
sudo vim /etc/redis/redis.conf
````

Change the following settings in the Redis configuration file:
```text
maxmemory 256mb
maxmemory-policy allkeys-lru
```

Restart Redis after the changes and enable it to start on boot:
```bash
sudo systemctl restart redis-server.service
sudo systemctl enable redis-server.service
redis-cli ping
```
