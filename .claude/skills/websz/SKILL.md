---
name: websz
description: Client for websz, a self-hosted web file manager. Browse, search, read, download, upload, move, and delete files on a running websz server, and get streaming URLs for its videos. Use this skill whenever the user mentions websz, gives a websz URL or token, or wants to transfer or manage files on a machine that serves them with websz, including when they just say "传到 websz", "从 websz 下载", "websz 上有什么", "看看服务器上的文件", or "get that file from my 18090 server". Do not use it to develop the websz server itself or for other file servers such as S3 or FTP.
allowed-tools: Bash Read
---

# websz client

Use `scripts/websz.py` (Python 3.8+, no dependencies) for every server operation, addressed by its full path inside this skill's directory. It covers guards that raw `curl` calls lack, described under "Deleting and overwriting".

## Connecting

Settings resolve in this order: flags (`--url`, `--token`), then environment variables (`WEBSZ_URL`, `WEBSZ_TOKEN`), then a `?t=<token>` in the URL. The default URL is `http://127.0.0.1:18090`.

Start with `info`, which verifies both connection and token and shows whether the server is read-only:

```bash
WEBSZ_URL=http://192.168.1.20:18090 WEBSZ_TOKEN=abc python3 scripts/websz.py info
```

Prefix each command with the variables, because shell state does not persist between Bash calls. If `WEBSZ_URL` isn't set and the user hasn't named a server, ask which one they mean rather than assuming the local default holds the files they want.

The token grants full access to the served directory. Don't repeat it in replies, write it into files, or use `url --with-token` unless the user wants a shareable link. websz has no TLS, so a token sent to a non-local `http://` address is readable on the network; mention this once if the server is reached across an untrusted network.

## Commands

Remote paths are rooted at the server's served directory, so `/` is that directory and not the machine's filesystem root.

| Task | Command |
|---|---|
| Check connection and mode | `info` |
| List a directory | `ls [/path] [--json]` |
| Search names recursively | `find QUERY [/path] [--limit N] [--json]` |
| Print a text file | `cat /path [--max-bytes N] [--offset N]` |
| Get full metadata and SHA-256 | `stat /path` |
| Download a file | `get /path [local] [--force]` |
| Download a directory as a zip | `get /dir [local.zip] [--force]` |
| Download and unpack a directory | `get /dir [localdir] --extract [--force]` |
| Upload a file | `put local /remote/name` or `put local /remote/dir/` |
| Upload several files | `put a b c /remote/dir` |
| Upload a directory | `put -r localdir /remote/dir` |
| Upload text from stdin | `printf 'hi' \| put - /remote/name` |
| Create a directory | `mkdir [-p] /path` |
| Rename or move | `mv /from /to [--overwrite]` |
| Delete | `rm [-r] /path...` |
| Get a streaming URL | `url /path [--with-token]` |

Every command also accepts `--url`, `--token`, and `--timeout SECONDS` (default 300). Raise the timeout for `stat` on very large files.

`put` also takes `-p` to create missing remote directories and `--conflict reject|skip|autorename|overwrite` for when the remote file exists. The default is `reject`.

## Choosing the right command

- **Prefer `ls`/`find --json` for size and mtime.** `stat` hashes the entire file on the server, so reserve it for when you need the checksum. Comparing it with a local `shasum -a 256` is the way to verify an important transfer.
- **Keep `cat` output small.** It returns the first 64 KB by default and states on stderr where to continue, which keeps large logs from flooding the conversation. For binary files it stops and points to `get`.
- **Watch for truncated searches.** `find` reports on stderr when results hit the limit, in which case the list is incomplete. Matching is a case-insensitive substring on names only, not contents, with no wildcards.
- **Directory zips omit symlinks** but include hidden files such as `.DS_Store`. `put -r` also skips symlinks and reports each one.

## Deleting and overwriting

The server has no trash and no undo, so:

- Before `rm`, `mv --overwrite`, `put --conflict overwrite`, or `get --force`, make sure the user asked for that specific outcome. If a request is vague ("clean up the old stuff"), list what would be affected and get confirmation.
- Before `rm -r`, run `ls` or `find` on the target and tell the user what it holds unless they have already seen it.
- Never work around the client's refusals with direct API calls. In particular, the server itself will wipe the entire served directory if asked to delete `/`, so the client's refusal is the only protection there.

## Playing media

`url /movies/clip.mp4` prints an `/open` URL that supports seeking. A player that can set headers should send the token that way:

```bash
mpv --http-header-fields="X-Websz-Token: $WEBSZ_TOKEN" "$(python3 scripts/websz.py url /movies/clip.mp4)"
```

Browsers and `<video>` tags can't set headers, so they need `url --with-token`. Tell the user that link contains the token.

## Errors

| Message | What to do |
|---|---|
| `cannot reach ...` | The server is down or the URL is wrong. Ask the user to check it. |
| `HTTP 401` | The token is missing or wrong. A non-localhost server without `-token` generates one at startup and logs it. |
| `write endpoint not available` | The server runs with `-readonly`. Writes need a restart without that flag, which is the user's decision. |
| `already exists` | The client refused to overwrite. Ask the user or pick a `--conflict`/`--force` option that matches their request. |
| `path traversal attempt` | The server rejects any path containing `..`, even in a name like `a..b.txt`. Such files can't be reached through the API; tell the user. |
| `HTTP 500` otherwise | Often filesystem permissions on the server. Report the message as is. |

Read `references/api.md` only if you need behaviour the script does not expose.
