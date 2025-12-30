# websz

A Go-based local directory file transfer and management web tool that provides a Windows-style file manager interface for your local files through a web browser.

## Features

- **Secure File Management**: Browse, upload, download, rename, and delete files with path traversal protection
- **Web Interface**: Clean, Windows-style file manager interface accessible through any web browser
- **File Preview**: Direct browser preview for images, videos (mp4), PDFs, and text files
- **Drag & Drop Upload**: Drag files directly from your desktop to upload
- **Context Menus**: Right-click context menus for file operations
- **Token Authentication**: Optional token-based authentication for network access
- **Read-only Mode**: Restrict to read-only operations when needed
- **Cross-platform**: Works on Windows, macOS, and Linux

## Quick Start

1. **Build the application:**
   ```bash
   go build -o websz ./cmd/websz
   ```

2. **Run with default settings:**
   ```bash
   ./websz
   ```

3. **Open your browser and visit:**
   ```
   http://localhost:18090
   ```

## Usage

### Command Line Options

```bash
websz [options]

Options:
  -root string
        Root directory (default: current working directory)
  -listen string
        Listen address (default "0.0.0.0:18090")
  -token string
        Access token (default: none, recommended for non-localhost)
  -readonly
        Read-only mode
  -help
        Show help
```

### Examples

```bash
# Serve current directory on localhost only
./websz -listen 127.0.0.1:8080

# Serve specific directory with token authentication
./websz -root /home/user/files -token mytoken123

# Read-only mode for safe browsing
./websz -readonly

# Serve on all interfaces with auto-generated token
./websz
```

## API Endpoints

The application provides RESTful API endpoints:

- `GET /api/list?p=/path` - List directory contents
- `GET /api/stat?p=/path` - Get file/directory properties  
- `GET /api/download?p=/path` - Download file
- `GET /open?p=/path` - Preview file in browser
- `POST /api/upload?p=/path` - Upload files (multipart form)
- `PUT /api/put?p=/path` - Upload single file (raw body)
- `POST /api/mkdir` - Create directory
- `POST /api/rename` - Rename/move files
- `POST /api/delete` - Delete files/directories

All API responses follow the format:
```json
{
  "ok": true,
  "data": {...},
  "error": ""
}
```

## Security Features

- **Path Traversal Protection**: Prevents access to files outside the root directory
- **Token Authentication**: Optional token-based access control
- **Safe File Operations**: Atomic file uploads and proper error handling
- **CORS Support**: Configurable cross-origin resource sharing

## File Preview Support

The following file types can be previewed directly in the browser:

- **Images**: PNG, JPG, JPEG, GIF, WebP
- **Videos**: MP4, MOV, AVI, WebM (with seek/range support)  
- **Documents**: PDF, TXT, Markdown
- **Other files**: Download only

## Frontend Features

- **File List View**: Windows-style details view with sortable columns
- **Breadcrumb Navigation**: Click to navigate to parent directories
- **File Operations**: Upload, download, rename, delete, create folders
- **Context Menus**: Right-click for file-specific actions
- **Drag & Drop**: Drag files from desktop to upload
- **File Properties**: View detailed file information

## Development

### Project Structure

```
cmd/websz/          # Main application entry point
internal/fs/        # File system operations and security
internal/server/    # HTTP server and API handlers  
web/dist/          # Frontend static files (HTML, CSS, JS)
```

### Security Testing

Run the security test to verify path traversal protection:

```bash
go run test_security.go
```

## License

This project is open source. Use it responsibly and ensure proper security measures when exposing to networks.

## Notes

- Default binding to `0.0.0.0:18090` allows network access - use token authentication
- Large file uploads are supported with streaming to prevent memory issues
- File conflicts during upload are auto-resolved with numbered suffixes
- Read-only mode disables all write operations for safe browsing