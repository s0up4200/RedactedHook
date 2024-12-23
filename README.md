# RedactedHook

> \[!NOTE]
>
> RedactedHook's functionality is now integrated directly into autobrr. RedactedHook was a Go-based companion service designed to enhance autobrr's filtering capabilities for RED and OPS, specifically by checking uploader names, and record labels associated with torrents. This feature is now a native part of autobrr.

RedactedHook is a webhook companion service for [autobrr](https://github.com/autobrr/autobrr) designed to check the names of uploaders, your ratio, torrent size and record labels associated with torrents on **Redacted** and **Orpheus**. It provides a simple and efficient way to validate if uploaders are blacklisted or whitelisted, to stop racing in case your ratio falls below a certain point, and to verify if a torrent's record label matches against a specified list.

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [Warning](#warning)
- [Installation](#installation)
  - [Docker](#docker)
  - [Docker Compose](#docker-compose)
  - [Using precompiled binaries](#using-precompiled-binaries)
  - [Building from source](#building-from-source)
- [Usage](#usage)
  - [Commands](#commands)
- [Config](#config)
- [Authorization](#authorization)
- [Payload](#payload)

## Features

- Verify if an uploader's name is on a provided whitelist or blacklist.
- Check for record labels. Useful for grabbing torrents from a specific record label.
- Check if a user's ratio meets a specified minimum value.
- Check the torrentSize (Useful for not hitting the API from both autobrr and redactedhook).
- Easy to integrate with other applications via webhook.
- Rate-limited to comply with tracker API request policies.
  - With a 5-minute data cache to reduce frequent API calls for the same data.

It was made with [autobrr](https://github.com/autobrr/autobrr) in mind.

## Getting Started

## Warning

> \[!IMPORTANT]
>
> Remember that autobrr also checks the RED/OPS API if you have min/max sizes set. This will result in you hitting the API 2x.
> So for your own good, **only** set size checks in RedactedHook.

## Installation

### Docker

```bash
docker pull ghcr.io/s0up4200/redactedhook:latest
```

### Docker Compose

```docker
services:
  redactedhook:
    container_name: redactedhook
    image: ghcr.io/s0up4200/redactedhook:latest
    user: 1000:1000
    #user: nobody
    #read_only: true
    #security_opt:
    #  - no-new-privileges:true
    #cap_drop:
    #  - ALL
    environment:
      #- REDACTEDHOOK__HOST=127.0.0.1   # Override the host from config.toml
      #- REDACTEDHOOK__PORT=42135       # Override the port from config.toml
      #- REDACTEDHOOK__API_TOKEN=       # Override the api_token from config.toml
      #- REDACTEDHOOK__RED_APIKEY=      # Override the red api_key from config.toml
      #- REDACTEDHOOK__OPS_APIKEY=      # Override the ops api_key from config.toml
      - TZ=UTC
    ports:
      - "42135:42135"
    volumes:
      - ./:/redactedhook
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/usr/local/bin/redactedhook", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Using precompiled binaries

Download the appropriate binary for your platform from the [releases](https://github.com/s0up4200/RedactedHook/releases/latest) page.

### Building from source

1. Clone the repository:

```bash
git clone https://github.com/s0up4200/RedactedHook.git
```

2. Navigate to the project directory:

```bash
cd RedactedHook
```

3. Build the project:

```bash
go build
or
make build
```

4. Run the compiled binary:

```bash
./bin/RedactedHook --config /path/to/config.toml # config flag not necessary if file is next to binary
```

## Usage

To use RedactedHook, send POST requests to the following endpoint:

```console
Endpoint: http://127.0.0.1:42135/hook
Header: X-API-Token: YOUR_API_TOKEN
Method: POST
Expected HTTP Status: 200
```

You can check ratio, uploader (whitelist and blacklist), minsize, maxsize, and record labels in a single request, or separately.

### Commands

- `generate-apitoken`: Generate a new API token and print it.
- `create-config`: Create a default configuration file.
- `help`: Display this help message.

## Config

Most of requestData can be set in config.toml to reduce the payload from autobrr.

### Example config.toml

```toml
[server]
host = "127.0.0.1" # Server host
port = 42135       # Server port

[authorization]
api_token = "" # generate with "redactedhook generate-apitoken"

[indexer_keys]
#red_apikey = "" # generate in user settings, needs torrent and user privileges
#ops_apikey = "" # generate in user settings, needs torrent and user privileges

[userid]
#red_user_id = 0 # from /user.php?id=xxx
#ops_user_id = 0 # from /user.php?id=xxx

[ratio]
#minratio = 0.6 # reject releases if you are below this ratio

[sizecheck]
#minsize = "100MB" # minimum size for checking, e.g., "10MB"
#maxsize = "500MB" # maximum size for checking, e.g., "1GB"

[uploaders]
#uploaders = "greatest-uploader" # comma separated list of uploaders to allow
#mode = "whitelist" # whitelist or blacklist

[record_labels]
#record_labels = "" # comma separated list of record labels to filter for

[logs]
loglevel = "trace"               # trace, debug, info
logtofile = false                # Set to true to enable logging to a file
logfilepath = "redactedhook.log" # Path to the log file
maxsize = 10                     # Max file size in MB
maxbackups = 3                   # Max number of old log files to keep
maxage = 28                      # Max age in days to keep a log file
compress = false                 # Whether to compress old log files
```

## Authorization

API Token can be generated like this: `redactedhook generate-apitoken`

Set it in the config, and use it as a header like:

![autobrr-external-filter-example](<.github/images/autobrr-external-filters.png>)

`CURL` if you want to test:

```bash
curl -X POST \
     -H "X-API-Token: 098qw0e98ass" \
     -H "Content-Type: application/json" \
     -d '{"torrent_id": 12345, "indexer": "ops", "uploaders": "the_worst_uploader,thebestuploader", "mode": "blacklist"}' \
     http://127.0.0.1:42135/hook
```

## Payload

The minimum required data to send with the webhook:

```json
{
    "torrent_id": {{.TorrentID}},
    "indexer": "{{ .Indexer | js }}"
}
```

Everything else can be set in the config.toml, but you can set them in the webhook as well, if you want to filter by different things in different filters.

- `indexer` - `"{{ .Indexer | js }}"` this is the indexer that pushed the release within autobrr.
- `torrent_id` - `{{.TorrentID}}` this is the TorrentID of the pushed release within autobrr.

### Additional Keys

- `red_user_id` is the number in the URL when you visit your profile.
- `ops_user_id` is the number in the URL when you visit your profile.
- `red_apikey` is your Redacted API key. Needs user and torrents privileges.
- `ops_apikey` is your Orpheus API key. Needs user and torrents privileges.
- `record_labels` is a comma-separated list of record labels to check against.
- `minsize` is the minimum allowed size you want to grab. Eg. 100MB
- `maxsize` is the max allowed size you want to grab. Eg. 500MB
- `uploaders` is a comma-separated list of uploaders to check against.
- `mode` is either blacklist or whitelist. If blacklist is used, the torrent will be stopped if the uploader is found in the list. If whitelist is used, the torrent will be stopped if the uploader is not found in the list.
  `
