# Kyber ChatFilter Discord Relay (Docker Image)

A lightweight Go-based Docker image (sidecar) that monitors Kyber server logs and forwards **ChatFilter moderation events** to Discord via webhooks.

Designed to run alongside a Kyber dedicated server container without modifying either the Kyber image or ChatFilter plugin.



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
```

The container's interaction with server logs is read-only.
It does not modify any data in the Kyber container in any way.


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



## Quick Start (Docker Run Example)

```bash
docker run -d \
  --name chatfilter-discord-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xxxxx" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="HvV Chaos #1" \
  -v /path/to/kyber/logs:/mnt/logs:ro \
  kyber-chatfilter-discord:latest
```

That’s it.



# Environment Variables

## Required

| Variable              | Description                    |
| --------------------- | ------------------------------ |
| `DISCORD_WEBHOOK_URL` | Default webhook for all events |

If this is not set, the container will exit.



## Optional Webhook Overrides

You can route different event types to different Discord channels:

| Variable                        | Event Type         |
| ------------------------------- | ------------------ |
| `DISCORD_WEBHOOK_DETECTION_URL` | Word detections    |
| `DISCORD_WEBHOOK_ACTION_URL`    | Mutes, kicks, bans |
| `DISCORD_WEBHOOK_ERROR_URL`     | Errors             |
| `DISCORD_WEBHOOK_INFO_URL`      | Informational logs |

If not set, events for variables not set fall back to `DISCORD_WEBHOOK_URL`.



## Optional Configuration Variables

| Variable             | Default        | Description                             |
| -------------------- | -------------- | --------------------------------------- |
| `LOG_DIR`            | `/mnt/logs`    | Directory containing Kyber logs         |
| `KYBER_SERVER_NAME`  | `Kyber Server` | Display name in Discord embed           |
| `RATE_LIMIT_SECONDS` | `5`            | Minimum seconds between same event type |
| `DISABLE_RATE_LIMIT` | `false`        | Disable internal rate limiting          |
| `LOG_POLL_INTERVAL`  | `500`          | Poll interval in milliseconds           |
| `ENABLE_DETECTION`   | `true`         | Send detection events                   |
| `ENABLE_ACTION`      | `true`         | Send moderation actions                 |
| `ENABLE_ERROR`       | `true`         | Send error events                       |
| `ENABLE_INFO`        | `true`         | Send info events                        |



## Event Types

The relay automatically classifies ChatFilter log lines:

| Prefix in Log | Event Type | Discord Embed Title  | Embed Color         |
|---------------|------------|----------------------|---------------------|
| `Detection:`  | detection  | ChatFilter Detection | 10038562 (Dark Red) |
| `Action:`     | action     | ChatFilter Action    | 16753920 (Orange)   |
| `Error:`      | error      | ChatFilter Error     | 15158332 (Red)      |
| *(no prefix)* | info       | ChatFilter Info      | 3447003 (Azure)     |

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

Each event type has its own timer.

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
  -v /srv/kyber/logs:/mnt/logs:ro \
  kyber-chatfilter-discord:latest
```

All events go to one Discord channel.



### Example 2: Separate Webhook Channels Per Event Type

```bash
docker run -d \
  --chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/default" \
  -e DISCORD_WEBHOOK_DETECTION_URL="https://discord.com/api/webhooks/detection" \
  -e DISCORD_WEBHOOK_ACTION_URL="https://discord.com/api/webhooks/action" \
  -e DISCORD_WEBHOOK_ERROR_URL="https://discord.com/api/webhooks/error" \
  -e LOG_DIR=/mnt/logs \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -v /srv/kyber/logs:/mnt/logs:ro \
  kyber-chatfilter-discord:latest
```

Detection logs → #detections  

Actions → #moderation-log  

Errors → #server-errors  

Misc → #info-log



### Example 3: High-Traffic Server Tuning

```bash
docker run -d \
  --chatfilter-relay \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/default" \
  -e LOG_DIR=/mnt/logs \
  -e RATE_LIMIT_SECONDS=10 \
  -e ENABLE_INFO=false \
  -e ENABLE_DETECTION=true \
  -e ENABLE_ACTION=true \
  -e KYBER_SERVER_NAME="My Kyber Server" \
  -v /srv/kyber/logs:/mnt/logs:ro \
  kyber-chatfilter-discord:latest
```

* Info logs disabled
* Detection logs enabled
* Action logs enabled
* Increased rate limiting
* Can still use multiple webhooks if needed



## Security Notes

* The container only reads logs (read-only mount recommended)
* No inbound ports are exposed
* No external API used other than Discord webhook
* Does not modify Kyber server container files

Always protect your webhook URLs.



## Performance Notes

* Uses polling (default 500ms)
* Lightweight memory usage
* Handles large log files
* Minimal CPU overhead
* Designed for high-chat-traffic Kyber servers



## Failure Behavior

* Missing webhook → container exits
* Missing log files → retries
* Discord errors → logs error and continues
* Log rotation → auto-detected



## Recommended Docker Volume Mount

Use read-only mount:

```bash
-v /path/to/kyber/logs:/mnt/logs:ro
```

Prevents accidental modification.


## Not Included

This relay:

* Does NOT modify Kyber server behavior
* Does NOT persist ChatFilter bans
* Does NOT read player state
* Does NOT require Kyber plugin HTTP support

It is strictly a log-forwarding service.


## Compatible With

* Kyber dedicated servers
* ChatFilter plugin
* Docker-based deployments
* Sidecar architecture



