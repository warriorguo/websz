# websz HTTP API notes

`docs/API.md` in the websz repository is the full reference. Authenticate with an `X-Websz-Token` header. JSON endpoints wrap results as `{"ok": true, "data": ...}` or `{"ok": false, "error": "..."}`.

| Method | Endpoint | Input |
|---|---|---|
| GET | `/api/session` | none; returns `token`, `readOnly` |
| GET | `/api/list` | `?p=` |
| GET | `/api/find` | `?p=&q=&limit=` |
| GET | `/api/stat` | `?p=` |
| GET | `/api/download` | `?p=`; a directory streams as a zip |
| GET | `/open` | `?p=`; inline with `Range` support |
| POST | `/api/upload` | `?p=&conflict=autorename\|overwrite\|reject`; multipart field `files` |
| PUT | `/api/put` | `?p=`; raw body |
| POST | `/api/mkdir` | `{"p": parent, "name": name}` |
| POST | `/api/rename` | `{"from": path, "to": path}` |
| POST | `/api/delete` | `{"p": path, "recursive": bool}` |

## Server behaviour worth knowing

Read from the server source. `scripts/websz.py` already handles each of these.

- `POST /api/delete` with `p` set to `/` and `recursive` true removes everything in the served directory.
- `PUT /api/put` and `POST /api/rename` silently replace an existing file.
- `PUT /api/put` does not create parent directories.
- `POST /api/rename` reports a missing destination directory as `Source file not found`.
- `POST /api/delete` needs `recursive` for every directory, including empty ones.
- `GET /api/find` falls back to 500 results when `limit` is above 5000, and returns `"items": null` when nothing matches.
- `POST /api/upload` treats an unknown `conflict` value as overwrite.
- On a `-readonly` server, write routes are unregistered, so writes get a plain-text 404 rather than the documented JSON 403.
