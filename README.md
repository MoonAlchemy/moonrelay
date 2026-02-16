# Moonrelay

A lightweight and efficient TikTok Live restreamer and archiver.

## Features

- **Efficient**: Uses `-c copy` to pass through streams without re-encoding, ensuring minimal resource usage.
- **Auto-Recovery**: Automatically detects when a user goes live and restarts the stream if it drops.
- **Easy to Use**: Just set the username and mode, and it handles the rest.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)

## Quick Start

### 1. Build the image
```bash
docker build -t moonrelay .
```

### 2. Run in Restream Mode (YouTube/Twitch)
```bash
docker run -d --name moonrelay \
  moonrelay \
  -user <tiktok_username> \
  -mode stream \
  -rtmp rtmp://a.rtmp.youtube.com/live2 \
  -key <your_stream_key>
```

### 3. Run in Auto-Archive Mode
```bash
docker run -d --name moonrelay \
  -v $(pwd)/recordings:/app \
  moonrelay \
  -user <tiktok_username> \
  -mode download
```

## CLI Arguments

| Flag | Description | Default |
|------|-------------|---------|
| `-user` | TikTok username (without @) | (Required) |
| `-mode` | `'download'` or `'stream'` | `download` |
| `-rtmp` | Destination RTMP URL | (Required for stream) |
| `-key` | RTMP Stream Key | (Required for stream) |

## Tech Stack

- **Go 1.25**
- **FFmpeg 8.0.1**

## License

MIT
