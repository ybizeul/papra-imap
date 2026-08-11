## Introduction

Papra-imap is a simple email collector for your [papra](https://github.com/papra-hq/papra) instance.

Configure IMAP accounts and a papra API key and papra-imap will start monitoring and send to
papra new documents.

## Configuration

Configuration is done in yaml :

```yaml
papra:
    host: https://mypapra.org/
    api_key: ppapi_abcdef
accounts:
    - name: gmail
        host: imap.gmail.com
        port: 993
        ssl: true
        username: myusername
        password: my_app_password
        email: yann+documents@gmail.com
        folder: INBOX
        organization_id: org_abcdef
        mark_as_read: true
        poll_interval: 5m
        extensions:
            - pdf
            - docx
```

### Parameters

#### `papra`

| key     | description                                                            | default |
|---------|------------------------------------------------------------------------|---------|
| host    | Base URL or hostname of your papra instance (scheme is optional).      | -       |
| api_key | API key used to authenticate uploads to papra.                         | -       |

#### `accounts[]`

| key             | description                                                                                      | default |
|-----------------|--------------------------------------------------------------------------------------------------|---------|
| name            | A name for the account (used in logs).                                                           | -       |
| host            | IMAP server hostname.                                                                            | -       |
| port            | IMAP server port.                                                                                | `993` when `ssl: true`, otherwise `143` |
| ssl             | Whether to connect with TLS (`true`) or insecure IMAP (`false`).                                | `true` when `port` resolves to `993`, otherwise `false` |
| username        | IMAP login username.                                                                             | -       |
| password        | IMAP login password (or app password).                                                           | -       |
| email           | Optional `To` filter. If set, only unseen messages addressed to this value are processed.       | empty (no recipient filter) |
| folder          | IMAP folder/mailbox to monitor.                                                                  | `INBOX` |
| organization_id | Target papra organization ID for uploaded attachments.                                           | -       |
| mark_as_read    | If `true`, marks a message as seen only when all matching attachments uploaded successfully.     | `false` |
| poll_interval   | Interval between checks. Used as poll interval or IDLE wake-up interval (for servers with IDLE).| `5m`    |
| extensions      | Allowed attachment extensions (with or without `.`). Omit/empty currently defaults to `pdf` only. | `["pdf"]` |

## Run As A Container

### Docker

Mount your config file into the container and pass `-config`:

```bash
docker run --rm \
    --name papra-imap \
    -v "$(pwd)/config.yaml:/config.yaml:ro" \
    ghcr.io/ybizeul/papra-imap:latest \
    -config /config.yaml
```

Add `-debug` to enable debug logs:

```bash
docker run --rm \
    --name papra-imap \
    -v "$(pwd)/config.yaml:/config.yaml:ro" \
    ghcr.io/ybizeul/papra-imap:latest \
    -config /config.yaml -debug
```

### Docker Compose

```yaml
services:
    papra-imap:
        image: ghcr.io/ybizeul/papra-imap:latest
        container_name: papra-imap
        restart: unless-stopped
        command: ["-config", "/config.yaml"]
        volumes:
            - ./config.yaml:/config.yaml:ro
```