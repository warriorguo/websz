# websz

A Go-based local directory file transfer and management web tool that provides a file manager interface through a web browser.

## Security Warning

> **This tool exposes your local filesystem to the network. Understand the risks before using it.**

- **Do NOT** run this tool on untrusted or public networks without token authentication.
- **Do NOT** serve sensitive directories (e.g., `/`, `~`, system directories) unless you know what you are doing.
- **Always** use `-token` when binding to non-localhost addresses.
- **Prefer** `-readonly` mode when you only need to browse or download files.
- **Prefer** `-listen 127.0.0.1:PORT` to restrict access to localhost only.
- This tool provides **no encryption** (no HTTPS). Use a reverse proxy with TLS if you need encrypted transport.
- Token authentication is **basic bearer-style** — it is not a substitute for proper access control in production environments.
- Be aware that anyone with the token has **full read/write access** to the served directory (unless `-readonly` is set).

## Features

- **File Management**: Browse, upload, download, rename, and delete files
- **Cinema Mode**: Fullscreen image/video viewer with keyboard and scroll navigation
- **Sortable Columns**: Click Name/Size/Modified headers to sort, state persists in URL
- **URL State Sync**: Directory path and sort order encoded in URL for easy sharing
- **Drag & Drop Upload**: Drag files from desktop to upload with progress bar
- **Token Authentication**: Optional token-based authentication
- **Read-only Mode**: Restrict to read-only operations
- **Path Traversal Protection**: All paths validated to stay within the root directory
- **Cross-platform**: macOS (Intel/Apple Silicon), Linux, Windows

## Install

Download a pre-built binary from [Releases](https://github.com/warriorguo/websz/releases), or build from source:

```bash
go build -o websz ./cmd/websz
```

## Usage

```bash
websz [options]

Options:
  -root string      Root directory (default: current working directory)
  -listen string    Listen address (default "0.0.0.0:18090")
  -token string     Access token (default: none, auto-generated for non-localhost)
  -readonly         Read-only mode
  -help             Show help
```

### Examples

```bash
# Serve current directory on localhost only (safest)
./websz -listen 127.0.0.1:8080

# Serve specific directory with token authentication
./websz -root /home/user/files -token mysecrettoken

# Read-only mode for safe browsing
./websz -root /data/media -readonly

# LAN access (token auto-generated, check console output)
./websz
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/list?p=/path` | List directory contents |
| GET | `/api/find?p=/path&q=query` | Recursive name search |
| GET | `/api/stat?p=/path` | Get file/directory info |
| GET | `/api/download?p=/path` | Download file (Range supported) |
| GET | `/open?p=/path` | Stream file inline (Range supported) |
| POST | `/api/upload?p=/path` | Upload files (multipart) |
| PUT | `/api/put?p=/path` | Upload single file (raw body) |
| POST | `/api/mkdir` | Create directory |
| POST | `/api/rename` | Rename/move file |
| POST | `/api/delete` | Delete file/directory |

All responses return JSON: `{"ok": true, "data": {...}, "error": ""}`.

See [docs/API.md](docs/API.md) for the full reference — auth, path semantics, `FileInfo` schema, status codes, and a worked example for building a video client.

## Project Structure

```
cmd/websz/          # Entry point, CLI flags
internal/fs/        # File system operations and path security
internal/server/    # HTTP server, routes, handlers
web/dist/           # Frontend static files (embedded at build time)
web/embed.go        # Embed FS export
```

## License

MIT
