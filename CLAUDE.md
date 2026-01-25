# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
# Build the application
go build -o websz ./cmd/websz

# Run with defaults (serves current directory on 0.0.0.0:18090)
./websz

# Common options
./websz -root /path/to/serve -listen 127.0.0.1:8080 -token mytoken -readonly
```

## Architecture

**websz** is a Go-based web file manager that serves local directories through a browser interface.

### Package Structure

- `cmd/websz/main.go` - Entry point. Parses CLI flags, initializes server config, embeds static files via `//go:embed web/dist`
- `internal/fs/` - File system operations with security layer
  - `PathHandler` validates and sanitizes paths (prevents traversal attacks)
  - `FileManager` wraps all file operations (list, stat, read, write, delete, rename)
- `internal/server/` - HTTP server and API handlers
  - `Server` struct holds config, file manager, and routes
  - Handlers map to RESTful endpoints under `/api/`
  - Auth via cookie (`websz_token`) or `X-Websz-Token` header

### Request Flow

1. Request hits `Server.ServeHTTP` which applies CORS and logging middleware
2. If token configured, auth check runs (bypassed for `/auth`, `/app.js`, `/favicon.ico`)
3. Routes dispatch to handlers in `handlers.go`
4. Handlers use `FileManager` methods, which call `PathHandler.SafePath()` to validate paths before any filesystem access

### Key Security Pattern

All file paths go through `PathHandler.SafePath()` which:
- URL-decodes and normalizes the path
- Rejects any path containing `..`
- Verifies the resolved absolute path stays within the root directory

### API Response Format

All API endpoints return JSON:
```json
{"ok": true, "data": {...}, "error": ""}
```

## Frontend

Static files in `web/dist/` are embedded at build time. The frontend is a vanilla JS Windows-style file manager (`app.js`).
