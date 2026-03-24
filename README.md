# Kyber ChatFilter Discord Relay (Docker Image)

A lightweight Go-based Docker image (sidecar) that monitors Kyber server logs and forwards **ChatFilter moderation events** to Discord via webhooks.

Designed to run alongside a Kyber dedicated server container without modifying either the Kyber image or ChatFilter plugin.

## Index

- [Overview](#overview)
- [Quick Start](#quick-start-docker-run-example)
- [Environment Variables](#environment-variables)
- [Persistence](#persistence)
- [Event Types](#event-types)
- [Rate Limiting](#rate-limiting)
- [Log Rotation](#log-rotation-handling)
- [Docker Run Examples](#example-docker-configurations)
- [Security & Performance](#security-notes)
- [Volume Mount Setup](#recommended-docker-volume-mounts)

## Overview

This relay:

* Watches the Kyber server log directory in real time
* Detects `[ChatFilter]` log entries
* Classifies events (Detection, Action, Error, Info)
* Sends structured Discord embeds via webhook
* Supports per-event webhooks
* Supports per-event rate limiting
* Automatically handles Discord 429 rate limits
* Detects log rotation automatically
* Allows persistence of ChatFilter moderation state (ban and mute lists) across restarts
* Syncs state with the ChatFilter plugin via HTTP
* Automatically restores state to ChatFilter after server restarts

This relay is **optional** and not required for the ChatFilter plugin to function.


## Architecture

```
Kyber Server (ChatFilter Plugin)
        ↓
Writes to kyber-server_*.log
        ↓
Discord Relay (this container)
        ↓
Discord Webhook

------------------------------

Persistence (optional)

ChatFilter HTTP API
    ↑        ↓
/state     /sync
    ↓        ↑
Local JSON state file

```

The container's interaction with server logs is read-only.
It does not modify any data in the Kyber container directly.


## Requirements

### Mandatory

* Docker
* A Discord webhook URL
* Access to the Kyber log directory

### Optional

* Separate webhooks per event type
* Custom server name
* Rate limiting controls
* Event type toggles
* Persistence (ban/mute list syncing)



## Quick Start (Docker Run Example)

```bash
docker run -d \
  --name chatfilter-discord-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xxxxx" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="HvV Chaos #1" \
  -v log_volume:/mnt/logs:ro \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

The docker image can be found at:
`ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest`

>⚠ **Important**: In order for the relay container to access the Kyber server logs, the Kyber server container 
> **must** be created with a volume mount for its log directory. Docker does not allow mounting new volumes to an 
> already existing container.  
> Example:
> ```bash
> -v log_volume:/root/.local/share/maxima/wine/prefix/drive_c/users/root/AppData/Roaming/ArmchairDevelopers/Kyber/Logs \
> ```


# Environment Variables

## Required

| Variable              | Description                    |
|-----------------------|--------------------------------|
| `DISCORD_WEBHOOK_URL` | Default webhook for all events |

If this is not set, the container will exit.



## Optional Webhook Overrides

You can route different event types to different Discord channels:

| Variable                        | Event Type         |
|---------------------------------|--------------------|
| `DISCORD_WEBHOOK_DETECTION_URL` | Word detections    |
| `DISCORD_WEBHOOK_ACTION_URL`    | Mutes, kicks, bans |
| `DISCORD_WEBHOOK_ERROR_URL`     | Errors             |
| `DISCORD_WEBHOOK_INFO_URL`      | Informational logs |

If not set, events for variables not set fall back to `DISCORD_WEBHOOK_URL`.



## Optional Configuration Variables

| Variable             | Default        | Description                             |
|----------------------|----------------|-----------------------------------------|
| `LOG_DIR`            | `/mnt/logs`    | Directory containing Kyber logs         |
| `KYBER_SERVER_NAME`  | `Kyber Server` | Display name in Discord embed           |
| `RATE_LIMIT_SECONDS` | `5`            | Minimum seconds between same event type |
| `DISABLE_RATE_LIMIT` | `false`        | Disable internal rate limiting          |
| `LOG_POLL_INTERVAL`  | `500`          | Poll interval in milliseconds           |
| `ENABLE_DETECTION`   | `true`         | Send detection events                   |
| `ENABLE_ACTION`      | `true`         | Send moderation actions                 |
| `ENABLE_ERROR`       | `true`         | Send error events                       |
| `ENABLE_INFO`        | `true`         | Send info events                        |


## Persistence

The relay optionally supports **state persistence for bans and mutes** using the ChatFilter plugin’s HTTP API.

### Features

* Periodically fetches state (ban/mute list) from ChatFilter (`/state`)
* Saves state to disk as JSON
* Restores state back to ChatFilter (`/sync`) on Kyber server restart

### Persistence Environment Variables

| Variable                | Default                 | Description                 |
|-------------------------|-------------------------|-----------------------------|
| `ENABLE_PERSISTENCE`    | `false`                 | Enable persistence system   |
| `STATE_FILE_PATH`       | `/mnt/state/state.json` | Path to stored state file   |
| `CHATFILTER_URL`        | `http://127.0.0.1:8081` | ChatFilter HTTP API URL     |
| `CHATFILTER_TOKEN`      | `CHANGE_ME_SECRET`      | HTTP auth token for `/sync` |
| `SYNC_INTERVAL_SECONDS` | `30`                    | Snapshot interval           |


### How It Works

#### Snapshot Loop

Every `SYNC_INTERVAL_SECONDS`:

```
GET /state → save to disk as state.json
```

#### Restore on Relay Startup

On relay container start:

```
Read state.json → POST /sync
```

#### 3. Automatic Resync on ChatFilter Restart

When this server log entry appears after a Kyber server restart:

```
[ChatFilter] Initialized plugin
```

The relay:

* Waits briefly for the ChatFilter plugin to fully initialize
* Restores ban/mute lists to ChatFilter automatically
* Retries sync up to 3 times if HTTP times out


### Volume Setup for Persistence

>⚠ **Important**: In order for the persistence feature to work properly and for state.json to survive relay
> container restarts/redeployments, the relay container **must** be created with a volume mount for its state
> directory. Docker does not allow mounting new volumes to an already existing container.  
> Example:
>```bash
>-v /home/username:/mnt/state
>```


## Event Types

The relay automatically classifies ChatFilter log lines:

| Prefix in Log | Event Type | Discord Embed Title  | Embed Color         |
|---------------|------------|----------------------|---------------------|
| `Detection:`  | detection  | ChatFilter Detection | 10038562 (Dark Red) |
| `Action:`     | action     | ChatFilter Action    | 16753920 (Orange)   |
| `Error:`      | error      | ChatFilter Error     | 15158332 (Red)      |
| *(no prefix)* | info       | ChatFilter Info      | 3447003 (Azure)     |

Any `[ChatFilter]` log entry that does not start with Detection:, Action:, or Error: is treated as an info event.

Example ChatFilter log lines:

```
[ChatFilter] Detection: PlayerOne (123456) used banned word 'bannedWord' [strike1/3]: used bannedWord in chat message
```

Will generate a Discord embed:

* Author: `KYBER_SERVER_NAME`
* Title: `ChatFilter Detection`
* Description: `PlayerOne (123456) used banned word 'bannedWord' [strike1/3]: used bannedWord in chat message`
* Color: Dark Red
* Timestamp: Current UTC time

```
[ChatFilter] Action: Admin banned PlayerOne (123456) for 30m
```

Will generate a Discord embed:

* Author: `KYBER_SERVER_NAME`
* Title: `ChatFilter Action`
* Description: `Admin banned PlayerOne (123456) for 30m`
* Color: Orange
* Timestamp: Current UTC time



## Rate Limiting

The image contains two layers of protection:

### Internal Rate Limiting

Each event type has its own timer. Rate limiting is applied per event type 
(detection, action, error, info), not per message content.

Default:

```
5 seconds between identical event types
```

Example:

* Detection events limited independently
* Action events limited independently

Disable with:

```
DISABLE_RATE_LIMIT=true
```

### Discord 429 Error Handling

If Discord responds with HTTP 429:

* The relay reads the `Retry-After` header
* Sleeps for the required duration
* Continues normally

Prevents crashes and embed spam.


## Log Rotation Handling

Kyber dedicated servers create new log files daily.

The relay:

* Detects when a new `kyber-server_*.log` appears
* Automatically switches to the newest file
* Continues monitoring without restart


## Example Docker Configurations


### Example 1: Simple Setup (Single Webhook Channel)

```bash
docker run -d \
  --name chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xxxxx" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -v log_volume:/mnt/logs:ro \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

All events go to one Discord channel.



### Example 2: Separate Webhook Channels Per Event Type

```bash
docker run -d \
  --name chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/default" \
  -e DISCORD_WEBHOOK_DETECTION_URL="https://discord.com/api/webhooks/detection" \
  -e DISCORD_WEBHOOK_ACTION_URL="https://discord.com/api/webhooks/action" \
  -e DISCORD_WEBHOOK_ERROR_URL="https://discord.com/api/webhooks/error" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -v log_volume:/mnt/logs:ro \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

Detection logs → #detections  

Actions → #moderation-log  

Errors → #server-errors  

Misc → #info-log



### Example 3: High-Traffic Server

```bash
docker run -d \
  --name chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/default" \
  -e LOG_DIR=/mnt/logs \
  -e RATE_LIMIT_SECONDS=10 \
  -e ENABLE_INFO=false \
  -e ENABLE_DETECTION=true \
  -e ENABLE_ACTION=true \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -v log_volume:/mnt/logs:ro \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

* Info logs disabled
* Detection logs enabled
* Action logs enabled
* Increased rate limiting
* Can still use multiple webhooks if needed


### Example 4: Simple Setup with Persistence Enabled

```bash
docker run -d \
  --name chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xxxxx" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -e ENABLE_PERSISTENCE=true \
  -e CHATFILTER_TOKEN="MY_AUTH_TOKEN"
  -v log_volume:/mnt/logs:ro \
  -v /home/username:/mnt/state \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

Enables ban/mute list persistence and sets the ChatFilter HTTP API auth token


### Example 5: Setup with Additional Persistence Options Set

```bash
docker run -d \
  --name chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xxxxx" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -e ENABLE_PERSISTENCE=true \
  -e CHATFILTER_TOKEN="MY_AUTH_TOKEN" \
  -e CHATFILTER_URL="http://192.168.0.1:8085" \
  -e SYNC_INTERVAL_SECONDS=15 \
  -v log_volume:/mnt/logs:ro \
  -v /home/username:/mnt/state \
  ghcr.io/leveldreadnought/kyber-chatfilter-discord:latest
```

* Enables ban/mute list persistence
* Sets the ChatFilter HTTP API auth token
* Changes the url and port of the ChatFilter plugin
* Decreases the state sync time


## Security Notes

* The relay container only reads logs (read-only mount recommended)
* Does not modify Kyber server container files
* Recommend a **trusted internal network** when persistence is enabled
* No external API used other than Discord webhook and ChatFilter's API
* Strongly recommend changing the default `CHATFILTER_TOKEN` value
* Avoid exposing the ChatFilter HTTP API publicly

Always protect your Discord webhook URLs.



## Performance Notes

* Uses polling (default 500ms)
* Lightweight memory usage
* Handles large log files
* Minimal CPU overhead
* No external dependencies
* Designed for high-chat-traffic Kyber servers



## Failure Behavior

* Missing webhook → container exits
* Missing log files → retries
* Discord errors → logs error and continues
* Log rotation → auto-detected



## Recommended Docker Volume Mounts

### ChatFilter Relay Container

Use read-only mount:

```bash
-v log_volume:/mnt/logs:ro
```

Prevents accidental modification.

#### Persistence

```bash
  -v /home/username:/mnt/state
```

Sets location of `state.json` on the host

### Kyber Dedicated Server Container

```bash
-v log_volume:/root/.local/share/maxima/wine/prefix/drive_c/users/root/AppData/Roaming/ArmchairDevelopers/Kyber/Logs \
```
Must be added on server container creation.


## Not Included

This relay:

* Does NOT modify Kyber server behavior
* Does NOT read player state

It is strictly a log-forwarding and persistence service.


## Compatible With

* Kyber dedicated servers
* ChatFilter plugin
* Docker-based deployments
* Docker Sidecar architecture



