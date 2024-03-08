---
title: Redis
layout: default
permalink: /docs/redis
parent: Configuring Muraena
---

# Redis

Muraena uses Redis as a caching backend for the tracking module. This section is used to configure the Redis settings.

## Settings

### `Host`
The `host` field specifies the hostname of the Redis server.

Default: `127.0.0.1`

### `Port`
The `port` field specifies the port of the Redis server.

Default: `6379`

### `Password`
The `password` field specifies the password of the Redis server. If not specified, it defaults to an empty string.

Default: ``



## Useful commands

### Reset the Redis cache

This command will reset the Redis cache, removing all the keys and their values, effectively clearing the cache.

> **NOTE**: This operation is irreversible and will remove all the data Muraena has stored in the cache, 
including tracking data and other useful information. Use with caution.


```bash
redis-cli FLUSHALL
```



