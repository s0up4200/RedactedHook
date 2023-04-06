RedactedHook is a webhook service designed to check user ratio and uploader names on Redacted. It provides a simple and efficient way to validate if a user has a specific minimum ratio or if an uploader is blacklisted.

## Features

- Check if a user's ratio meets a specified minimum value
- Verify if an uploader's name is on a provided blacklist
- Easy to integrate with other applications via webhook
- Works great with [autobrr](https://github.com/autobrr/autobrr)!

## Getting Started

### Prerequisites

To run RedactedHook, you'll need:

1. Go 1.20 or later installed (only if building from source)
2. Access to Redacted

### Installation

#### Using precompiled binaries

Download the appropriate binary for your platform from the [releases](https://github.com/s0up4200/RedactedHook/releases/latest) page.

#### Building from source

Clone the repository:

```bash
git clone https://github.com/s0up4200/RedactedHook.git
```

Navigate to the project directory:

```bash
cd RedactedHook
```
Build the project:

```go
go build
```

Run the compiled binary:

```bash
./RedactedHook
```

The RedactedHook server will now be running on port `42135`.

### Usage

To use RedactedHook, send POST requests to the following endpoints:

#### Check Ratio

- Endpoint: `http://127.0.0.1:42135/redacted/ratio`
- Method: POST
- Expected HTTP Status: 200

**JSON Payload:**

```json
{
"user_id": "USER_ID",
"apikey": "API_KEY",
"minratio": "MINIMUM_RATIO"
}
```

#### Check Uploader

- Endpoint: `http://127.0.0.1:42135/redacted/uploader`
- Method: POST
- Expected HTTP Status: 200

**JSON Payload:**

```json

{
  "id": "{{.TorrentID}}",
  "apikey": "API_KEY",
  "uploaders": "BLACKLISTED_USER1,BLACKLISTED_USER2,BLACKLISTED_USER3"
}
```