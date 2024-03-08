---
title: Log
permalink: /docs/log
nav_order: 6
parent: Configuring Muraena
---

# Log

The `log` section is used to configure the Muraena logging settings.


## Settings

### Enabled
When `enabled` is set to `true`, Muraena will log application events to the specified log file.

### File path
The `filePath` field specifies the path to the log file. If not specified, it defaults to `muraena.log` in the current 
working directory.

## Example

```toml
[log]
enabled = true
filePath = "/var/log/muraena.log"
```
