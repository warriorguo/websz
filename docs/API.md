# websz HTTP API

Reference for clients that want to talk to a `websz` server (e.g. a local video client that lists, searches, and plays media).

- Base URL: whatever you launched with (`-listen`), e.g. `http://127.0.0.1:18090`.
- All responses are JSON unless the endpoint streams file bytes (`/open`, `/api/download`).
- No HTTPS. Run behind a reverse proxy if you need TLS.

## Response envelope

JSON endpoints always wrap their payload:

```json
{ "ok": true,  "data": { ... } }
{ "ok": false, "error": "message" }
```

`data` contains the endpoint-specific payload described below. On error, HTTP status is non-2xx and `error` is set.

## Authentication

Required only when the server was started with `-token`. If no token is configured, every request is allowed.

A request is authenticated if **any** of the following matches the configured token:

| Method | Where | Notes |
|---|---|---|
| Cookie | `websz_token=<token>` | Set by `POST /auth`; lifetime 24h. |
| Header | `X-Websz-Token: <token>` | Recommended for programmatic clients. |
| Query  | `?t=<token>` | For shareable/self-authenticating URLs. The server will set the cookie on first use; for browser navigation it then redirects to a URL with `?t=` stripped. |

For API endpoints (`/api/*`) and `/open`, an unauthenticated request returns `401 { "error": "Authentication required" }`. Other paths redirect to `/auth`.

### `POST /auth`

Exchange a token for a session cookie. Useful for browser-style clients; programmatic clients can skip this and just send `X-Websz-Token` on every request.

Request:
```json
{ "token": "abcdef" }
```

Response (200): sets `websz_token` cookie.
```json
{ "ok": true, "data": { "message": "Authentication successful" } }
```

### `GET /api/session`

Returns the configured server token. Requires the request itself to already be authenticated. Useful for the web UI — third-party clients usually don't need this.

```json
{ "ok": true, "data": { "token": "abcdef" } }
```

## Path semantics

All file-operation endpoints take a virtual path via `?p=`:

- Paths are rooted at the server's `-root`. `/` is the root, `/movies/clip.mp4` resolves to `<root>/movies/clip.mp4`.
- Paths must be URL-encoded (`encodeURIComponent` in JS).
- Any path containing `..` is rejected with `400`.
- The resolved absolute path must stay inside the root directory.

## FileInfo schema

Returned (as an object or array element) by `/api/list`, `/api/find`, and `/api/stat`.

```ts
{
  name:   string;       // base name, e.g. "clip.mp4"
  path:   string;       // virtual path, e.g. "/movies/clip.mp4"
  isDir:  boolean;
  size:   number;       // bytes
  mtime:  string;       // RFC 3339 timestamp
  mime?:  string;       // e.g. "video/mp4"; only for non-dirs
  ext?:   string;       // lowercased extension incl. dot, e.g. ".mp4"
  mode?:  string;       // unix permission string; only on /api/stat
  sha256?: string;      // only on /api/stat (full file hash, may be slow)
  etag?:  string;       // quoted ETag; SHA256-derived on /stat, size+mtime on streaming endpoints
}
```

`/api/list` sorts directories first, then files (server-side). Within each group, current order matches `os.ReadDir` (filesystem order). Re-sort client-side if you need a specific order.

---

## Read endpoints

### `GET /api/list?p=<dir>`

List a directory's immediate children. `p` defaults to `/`.

```json
{
  "ok": true,
  "data": {
    "items": [ FileInfo, ... ],
    "root":  "/Users/me/Movies"   // absolute server-side root path (informational)
  }
}
```

Errors: `404` if the path does not exist; `400` for invalid paths.

### `GET /api/find?p=<dir>&q=<query>&limit=<n>`

Recursive case-insensitive substring match on file/directory names within `p`.

| Param | Default | Notes |
|---|---|---|
| `p` | `/` | Subtree to search. |
| `q` | _required_ | Substring (lowercased server-side). 400 if missing. |
| `limit` | `500` | Max 5000. Truncates depth-first walk when reached. |

```json
{
  "ok": true,
  "data": {
    "items":     [ FileInfo, ... ],
    "truncated": false,
    "query":     "blade",
    "root":      "/movies"
  }
}
```

`items[].mime` and `ext` are populated; `mode`/`sha256`/`etag` are not (use `/api/stat` for those).

### `GET /api/stat?p=<path>`

Full metadata for a single file or directory, including SHA-256 (computed on demand — can be slow for large files).

```json
{ "ok": true, "data": FileInfo }
```

### `GET /open?p=<path>`

Stream a file inline (`Content-Disposition: inline`). **This is the endpoint to use for video playback.**

- `Content-Type` is set from the file extension / sniffed.
- Backed by `http.ServeContent`, so:
  - `Range` requests are supported → the player can seek.
  - `If-None-Match` / `If-Modified-Since` short-circuit to `304`.
  - `ETag` is `"<size-hex>-<mtime-nanos-hex>"` (cheap, no SHA-256).
- Returns `400` if `p` points at a directory.

Example seek request:

```
GET /open?p=%2Fmovies%2Fclip.mp4
Range: bytes=1048576-
X-Websz-Token: abcdef
```

### `GET /api/download?p=<path>`

For files: same as `/open` but with `Content-Disposition: attachment` so browsers trigger a save. Also supports `Range`. Programmatic clients usually want `/open`.

For directories: streams a zip archive of the directory's contents.

- `Content-Type: application/zip`
- `Content-Disposition: attachment; filename="<dirname>.zip"`
- No `Content-Length` (the archive is streamed; size is not known in advance).
- `Range` is not supported for directories.
- Symlinks inside the tree are skipped (avoids loops and out-of-root targets). All other entries — including hidden files like `.DS_Store` — are included.
- Allowed in `-readonly` mode.

---

## Write endpoints (disabled when `-readonly`)

In read-only mode these all return `403 { "error": "Server is in read-only mode" }`.

### `POST /api/upload?p=<dir>&conflict=<strategy>`

`multipart/form-data` upload. Form field name: `files` (one or many).

`conflict` values:
- `autorename` (default) — appends `(1)`, `(2)`, … to the basename.
- `overwrite` — replace existing file in place.
- `reject` — `409 Conflict` if the name is taken.

```json
{ "ok": true, "data": { "uploaded": ["/dir/file1.mp4", "/dir/file2.mp4"] } }
```

### `PUT /api/put?p=<path>`

Streaming raw-body upload to a single path. The body is the file content.

```json
{ "ok": true, "data": { "path": "/dir/file.mp4" } }
```

`400` if `p` is `/`.

### `POST /api/mkdir`

```json
{ "p": "/parent", "name": "newdir" }
```

Response:
```json
{ "ok": true, "data": { "path": "/parent/newdir" } }
```

`409` if directory already exists.

### `POST /api/rename`

```json
{ "from": "/old/path.mp4", "to": "/new/path.mp4" }
```

Response:
```json
{ "ok": true, "data": { "from": "/old/path.mp4", "to": "/new/path.mp4" } }
```

Use to move as well as rename — `to` can be in a different directory.

### `POST /api/delete`

```json
{ "p": "/path", "recursive": false }
```

`recursive` must be `true` for non-empty directories; otherwise the server returns an error.

```json
{ "ok": true, "data": { "deleted": "/path" } }
```

---

## Status codes

| Code | When |
|---|---|
| 200 | Success. |
| 304 | `Range`/conditional request matched (`/open`, `/api/download`). |
| 400 | Invalid path, missing required param, directory passed where file expected. |
| 401 | Token required and missing/incorrect. |
| 403 | Read-only server. |
| 404 | Path does not exist. |
| 405 | Wrong method. `Allow` header lists accepted methods. |
| 409 | Conflict (existing file under `reject`, existing directory on `mkdir`). |
| 500 | Filesystem or server error. |

## CORS

`Access-Control-Allow-Origin: *` is set on every response, and `OPTIONS` preflights succeed for all endpoints. A browser-based client on a different origin can call the API directly, but token auth via cookie won't work cross-origin — use the `X-Websz-Token` header instead.

---

## Quick-start: a minimal video client

The cinema-mode UI in `web/dist/app.js` is a working example. The pattern is:

1. List media files in the current directory:

   ```bash
   curl -H 'X-Websz-Token: abcdef' \
     'http://127.0.0.1:18090/api/list?p=%2Fmovies'
   ```

   Filter the result client-side by `ext` against your supported video extensions. The bundled cinema mode treats these as video: `.mp4`, `.webm`, `.mov`, `.ogg`.

2. Optional: search across the whole tree.

   ```bash
   curl -H 'X-Websz-Token: abcdef' \
     'http://127.0.0.1:18090/api/find?p=%2F&q=trailer&limit=200'
   ```

3. Play a video — point any media player at the `/open` URL.

   ```bash
   mpv --http-header-fields='X-Websz-Token: abcdef' \
     'http://127.0.0.1:18090/open?p=%2Fmovies%2Fclip.mp4'
   ```

   Or with a query-param token (works in `<video>` tags and players that don't let you set headers):

   ```
   http://127.0.0.1:18090/open?p=%2Fmovies%2Fclip.mp4&t=abcdef
   ```

4. For an HTML5 `<video>` element:

   ```html
   <video src="http://127.0.0.1:18090/open?p=%2Fmovies%2Fclip.mp4&t=abcdef"
          controls autoplay></video>
   ```

   The browser will issue `Range` requests for seeking; the server handles them via `http.ServeContent`.
