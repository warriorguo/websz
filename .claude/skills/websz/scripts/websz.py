#!/usr/bin/env python3
"""Command-line client for a websz server. Standard library only.

Connection settings come from --url/--token or WEBSZ_URL/WEBSZ_TOKEN. A URL
carrying ?t=<token> also works. Results go to stdout, errors to stderr, and the
exit code is non-zero on failure.

Beyond wrapping the HTTP API, this client adds guards the server does not have:
it never deletes or moves the root, and it refuses to overwrite remote or local
files unless told to.
"""
from __future__ import annotations

import argparse
import json
import os
import posixpath
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import zipfile

DEFAULT_URL = "http://127.0.0.1:18090"
CHUNK = 1024 * 1024
FIND_MAX = 5000  # The server silently falls back to 500 above this.


class ClientError(Exception):
    def __init__(self, message, status=None):
        super().__init__(message)
        self.status = status


def norm(path):
    """Return the canonical virtual path, rejecting '..' components."""
    path = (path or "/").replace("\\", "/")
    if ".." in path.split("/"):
        raise ClientError(f"path must not contain '..': {path}")
    return posixpath.normpath("/" + path.lstrip("/"))


def join(parent, name):
    return posixpath.join(parent, name)


def parse_connection(url, token):
    """Resolve base URL and token, accepting URLs like http://host/?t=abc."""
    url = url or os.environ.get("WEBSZ_URL") or DEFAULT_URL
    if "://" not in url:
        url = "http://" + url
    parts = urllib.parse.urlsplit(url)
    url_token = urllib.parse.parse_qs(parts.query).get("t", [None])[0]
    base = urllib.parse.urlunsplit((parts.scheme, parts.netloc, parts.path.rstrip("/"), "", ""))
    return base, token or os.environ.get("WEBSZ_TOKEN") or url_token


class Client:
    def __init__(self, base, token, timeout):
        self.base = base
        self.token = token
        self.timeout = timeout

    def open_url(self, path):
        return f"{self.base}/open?p={urllib.parse.quote(path, safe='')}"

    def request(self, method, endpoint, query=None, data=None, headers=None):
        url = self.base + endpoint
        if query:
            url += "?" + urllib.parse.urlencode(query)
        req = urllib.request.Request(url, data=data, method=method, headers=headers or {})
        if self.token:
            req.add_header("X-Websz-Token", self.token)
        try:
            return urllib.request.urlopen(req, timeout=self.timeout)
        except urllib.error.HTTPError as err:
            raise self._http_error(err, method) from None
        except (urllib.error.URLError, OSError) as err:
            reason = getattr(err, "reason", err)
            raise ClientError(f"cannot reach {self.base}: {reason}") from None

    def _http_error(self, err, method):
        body = err.read(4096).decode("utf-8", "replace").strip()
        is_json = True
        try:
            message = json.loads(body).get("error") or body
        except (ValueError, AttributeError):
            message, is_json = body, False
        if err.code == 401:
            message += " (set WEBSZ_TOKEN or pass --token)"
        elif err.code == 404 and not is_json and method != "GET":
            # Write routes are not registered on -readonly servers, so these
            # requests reach the static file handler and get a plain-text 404.
            message = "write endpoint not available; the server is probably running with -readonly"
        return ClientError(f"HTTP {err.code}: {message}", status=err.code)

    def api(self, method, endpoint, query=None, body=None, data=None, headers=None):
        headers = dict(headers or {})
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        with self.request(method, endpoint, query, data, headers) as resp:
            raw = resp.read()
        try:
            envelope = json.loads(raw)
        except ValueError:
            raise ClientError(f"{self.base} did not return JSON; is this a websz server?") from None
        if not envelope.get("ok"):
            raise ClientError(envelope.get("error") or "server reported failure")
        return envelope.get("data")

    def list(self, path):
        return self.api("GET", "/api/list", {"p": path})["items"] or []

    def lookup(self, path):
        """Return a path's FileInfo, or None if it does not exist.

        Lists the parent rather than calling /api/stat, which hashes the whole
        file and can take minutes on large media.
        """
        if path == "/":
            return {"name": "", "path": "/", "isDir": True}
        try:
            items = self.list(posixpath.dirname(path))
        except ClientError as err:
            if err.status == 404:
                return None
            raise
        name = posixpath.basename(path)
        return next((item for item in items if item["name"] == name), None)

    def mkdirs(self, path):
        current = "/"
        for segment in [s for s in path.split("/") if s]:
            parent, current = current, join(current, segment)
            try:
                self.api("POST", "/api/mkdir", body={"p": parent, "name": segment})
            except ClientError as err:
                if err.status != 409:
                    raise
                info = self.lookup(current)
                if not (info and info["isDir"]):
                    raise ClientError(f"not a directory: {current}") from None


def print_items(items, as_json):
    if as_json:
        print(json.dumps(items, ensure_ascii=False, indent=2))
        return
    for item in items:
        kind = "dir " if item["isDir"] else "file"
        size = "-" if item["isDir"] else item["size"]
        print(f"{kind}  {size:>12}  {item['mtime'][:19]}  {item['path']}")


def warn(message):
    print(message, file=sys.stderr)


def cmd_info(client, args):
    session = client.api("GET", "/api/session")
    listing = client.api("GET", "/api/list", {"p": "/"})
    read_only = session.get("readOnly")
    # /api/session also returns the token; it is deliberately not printed.
    print(json.dumps({
        "url": client.base,
        "root": listing["root"],
        "tokenRequired": bool(session.get("token")),
        "readOnly": "unknown (server too old to report it)" if read_only is None else read_only,
    }, ensure_ascii=False, indent=2))


def cmd_ls(client, args):
    items = sorted(client.list(norm(args.path)), key=lambda i: (not i["isDir"], i["name"].lower()))
    print_items(items, args.json)


def cmd_find(client, args):
    data = client.api("GET", "/api/find", {"p": norm(args.path), "q": args.query, "limit": args.limit})
    print_items(data["items"] or [], args.json)
    if data.get("truncated"):
        warn(f"[truncated at {args.limit} results; raise --limit (max {FIND_MAX}) or narrow the search]")


def cmd_stat(client, args):
    print(json.dumps(client.api("GET", "/api/stat", {"p": norm(args.path)}), ensure_ascii=False, indent=2))


def cmd_cat(client, args):
    path = norm(args.path)
    last = args.offset + args.max_bytes - 1
    try:
        with client.request("GET", "/open", {"p": path}, headers={"Range": f"bytes={args.offset}-{last}"}) as resp:
            chunk = resp.read(args.max_bytes)
            content_range = resp.headers.get("Content-Range", "")
            full_length = resp.headers.get("Content-Length")
    except ClientError as err:
        if err.status == 416:  # Empty file, or offset at or past the end.
            return
        raise
    if b"\0" in chunk:
        raise ClientError(f"{path} looks binary; use 'get' to save it instead")
    sys.stdout.write(chunk.decode("utf-8", "replace"))
    sys.stdout.flush()
    total = content_range.rsplit("/", 1)[-1] if content_range else full_length
    if total and total.isdigit() and args.offset + len(chunk) < int(total):
        warn(f"\n[truncated: bytes {args.offset}-{args.offset + len(chunk) - 1} of {total}; "
             f"continue with --offset {args.offset + len(chunk)}]")


def download(client, path, dest_dir):
    """Stream /api/download to a temp file, checking length. Returns (temp, size)."""
    fd, tmp = tempfile.mkstemp(prefix=".websz-", suffix=".part", dir=dest_dir)
    written = 0
    try:
        with os.fdopen(fd, "wb") as out, client.request("GET", "/api/download", {"p": path}) as resp:
            expected = resp.headers.get("Content-Length")
            while True:
                chunk = resp.read(CHUNK)
                if not chunk:
                    break
                out.write(chunk)
                written += len(chunk)
        if expected is not None and written != int(expected):
            raise ClientError(f"incomplete download of {path}: got {written} of {expected} bytes")
    except BaseException:
        os.unlink(tmp)
        raise
    return tmp, written


def check_zip(tmp, path):
    # Directory zips are streamed with no Content-Length, so this is the only
    # way to notice the server dying mid-transfer.
    try:
        with zipfile.ZipFile(tmp) as archive:
            bad = archive.testzip()
    except zipfile.BadZipFile:
        raise ClientError(f"zip of {path} is incomplete or corrupt") from None
    if bad:
        raise ClientError(f"zip of {path} has a corrupt entry: {bad}")


def cmd_get(client, args):
    path = norm(args.remote)
    info = client.lookup(path)
    if info is None:
        raise ClientError(f"not found: {path}")
    name = posixpath.basename(path) or "websz-root"
    if args.extract and not info["isDir"]:
        raise ClientError("--extract only applies to directories")

    if args.extract:
        dest = args.local or name
        if os.path.exists(dest) and (not os.path.isdir(dest) or (os.listdir(dest) and not args.force)):
            raise ClientError(f"{dest} exists and is not empty; pass --force to extract into it")
    else:
        leaf = name + ".zip" if info["isDir"] else name
        dest = args.local or leaf
        if os.path.isdir(dest):
            dest = os.path.join(dest, leaf)
        if os.path.exists(dest) and not args.force:
            raise ClientError(f"{dest} already exists; pass --force to overwrite")

    parent = os.path.dirname(os.path.abspath(dest))
    if not os.path.isdir(parent):
        raise ClientError(f"local directory does not exist: {parent}")
    tmp, size = download(client, path, parent)
    try:
        if info["isDir"]:
            check_zip(tmp, path)
        if args.extract:
            with zipfile.ZipFile(tmp) as archive:
                archive.extractall(dest)  # zipfile strips '..' and absolute paths.
            print(f"extracted {path} to {dest}")
        else:
            os.chmod(tmp, 0o666 & ~current_umask())
            os.replace(tmp, dest)
            tmp = None
            print(f"saved {dest} ({size} bytes)")
    finally:
        if tmp:
            os.unlink(tmp)


def current_umask():
    mask = os.umask(0)
    os.umask(mask)
    return mask


def autorename(client, path):
    """Pick a free name using the same 'name(1).ext' pattern as the server."""
    stem, ext = posixpath.splitext(path)
    for i in range(1, 1000):
        candidate = f"{stem}({i}){ext}"
        if client.lookup(candidate) is None:
            return candidate
    raise ClientError(f"no free name found for {path}")


def put_one(client, source, remote, conflict):
    existing = client.lookup(remote)
    if existing:
        if existing["isDir"]:
            raise ClientError(f"remote path is a directory: {remote}")
        if conflict == "skip":
            print(f"skipped {remote} (already exists)")
            return
        if conflict == "reject":
            raise ClientError(f"{remote} already exists; choose --conflict overwrite, autorename or skip")
        if conflict == "autorename":
            remote = autorename(client, remote)

    headers = {"Content-Type": "application/octet-stream"}
    if source == "-":
        client.api("PUT", "/api/put", {"p": remote}, data=sys.stdin.buffer.read(), headers=headers)
    else:
        headers["Content-Length"] = str(os.path.getsize(source))
        with open(source, "rb") as body:
            client.api("PUT", "/api/put", {"p": remote}, data=body, headers=headers)
    print(f"uploaded {remote}")


def walk_tree(source, remote):
    """Yield (local_file, remote_file) and (None, remote_dir) entries for a tree."""
    yield None, remote
    for current, dirs, files in os.walk(source):
        rel = os.path.relpath(current, source)
        base = remote if rel == "." else join(remote, rel.replace(os.sep, "/"))
        # Symlinks are skipped, matching the server's zip behaviour, so nothing
        # outside the tree being uploaded gets sent by accident.
        for name in [d for d in dirs if os.path.islink(os.path.join(current, d))]:
            dirs.remove(name)
            warn(f"skipped symlink {os.path.join(current, name)}")
        for name in dirs:
            yield None, join(base, name)
        for name in files:
            local = os.path.join(current, name)
            if os.path.islink(local):
                warn(f"skipped symlink {local}")
            elif os.path.isfile(local):
                yield local, join(base, name)
            else:
                warn(f"skipped special file {local}")


def cmd_put(client, args):
    target = norm(args.remote)
    info = client.lookup(target)
    if info and not info["isDir"] and (len(args.local) > 1 or args.remote.endswith("/")):
        raise ClientError(f"remote path is a file, not a directory: {target}")
    into_dir = bool(info and info["isDir"]) or args.remote.endswith("/") or len(args.local) > 1

    if into_dir and info is None:
        if not (args.parents or args.recursive):
            raise ClientError(f"remote directory does not exist: {target}; pass -p to create it")
        client.mkdirs(target)

    for source in args.local:
        if source == "-":
            if into_dir:
                raise ClientError("stdin needs a full remote file path, not a directory")
            remote = target
        else:
            if not os.path.exists(source):
                raise ClientError(f"local path not found: {source}")
            name = os.path.basename(os.path.abspath(source))
            remote = join(target, name) if into_dir else target

        if source != "-" and os.path.isdir(source):
            if not args.recursive:
                raise ClientError(f"{source} is a directory; pass -r to upload it")
            for local, dest in walk_tree(source, remote):
                if local is None:
                    client.mkdirs(dest)
                else:
                    put_one(client, local, dest, args.conflict)
            continue

        parent = posixpath.dirname(remote)
        if not into_dir and args.parents:
            client.mkdirs(parent)
        elif client.lookup(parent) is None:
            raise ClientError(f"remote directory does not exist: {parent}; pass -p to create it")
        put_one(client, source, remote, args.conflict)


def cmd_mkdir(client, args):
    path = norm(args.path)
    if path == "/":
        raise ClientError("the root already exists")
    if args.parents:
        client.mkdirs(path)
    else:
        parent, name = posixpath.split(path)
        client.api("POST", "/api/mkdir", body={"p": parent, "name": name})
    print(f"created {path}")


def cmd_mv(client, args):
    source, target = norm(args.source), norm(args.target)
    if source == "/":
        raise ClientError("refusing to move the root directory")
    if client.lookup(source) is None:
        raise ClientError(f"not found: {source}")
    info = client.lookup(target)
    if args.target.endswith("/") or (info and info["isDir"]):
        if info is None:
            raise ClientError(f"remote directory does not exist: {target}")
        target = join(target, posixpath.basename(source))
        info = client.lookup(target)
    if client.lookup(posixpath.dirname(target)) is None:
        # The server reports this as "Source file not found", which misleads.
        raise ClientError(f"remote directory does not exist: {posixpath.dirname(target)}")
    if info and source != target and not args.overwrite:
        raise ClientError(f"{target} already exists; pass --overwrite to replace it")
    client.api("POST", "/api/rename", body={"from": source, "to": target})
    print(f"moved {source} -> {target}")


def cmd_rm(client, args):
    paths = [norm(p) for p in args.path]
    if "/" in paths:
        # The server would run os.RemoveAll on the entire served directory.
        raise ClientError("refusing to delete the root directory")
    for path in paths:
        client.api("POST", "/api/delete", body={"p": path, "recursive": args.recursive})
        print(f"deleted {path}")


def cmd_url(client, args):
    url = client.open_url(norm(args.path))
    if args.with_token and client.token:
        url += "&t=" + urllib.parse.quote(client.token, safe="")
    print(url)


def limit_type(value):
    number = int(value)
    if not 1 <= number <= FIND_MAX:
        raise argparse.ArgumentTypeError(f"must be between 1 and {FIND_MAX}")
    return number


def positive_int(value):
    number = int(value)
    if number < 1:
        raise argparse.ArgumentTypeError("must be at least 1")
    return number


def non_negative_int(value):
    number = int(value)
    if number < 0:
        raise argparse.ArgumentTypeError("must not be negative")
    return number


def build_parser():
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--url", help=f"server URL (env WEBSZ_URL, default {DEFAULT_URL})")
    common.add_argument("--token", help="access token (prefer env WEBSZ_TOKEN)")
    common.add_argument("--timeout", type=float, default=300, help="network timeout in seconds (default 300)")

    parser = argparse.ArgumentParser(prog="websz.py", description="Client for a websz file server.")
    sub = parser.add_subparsers(dest="command", required=True)

    def add(name, func, help_text):
        command = sub.add_parser(name, parents=[common], help=help_text, description=help_text)
        command.set_defaults(func=func)
        return command

    add("info", cmd_info, "check the connection; show the root and whether the server is read-only")

    p = add("ls", cmd_ls, "list a directory")
    p.add_argument("path", nargs="?", default="/")
    p.add_argument("--json", action="store_true", help="print full FileInfo objects")

    p = add("find", cmd_find, "recursive case-insensitive substring search on names")
    p.add_argument("query")
    p.add_argument("path", nargs="?", default="/", help="subtree to search (default /)")
    p.add_argument("--limit", type=limit_type, default=500, help=f"max results, up to {FIND_MAX} (default 500)")
    p.add_argument("--json", action="store_true", help="print full FileInfo objects")

    p = add("stat", cmd_stat, "full metadata including SHA-256 (hashes the whole file; slow on large files)")
    p.add_argument("path")

    p = add("cat", cmd_cat, "print part of a text file")
    p.add_argument("path")
    p.add_argument("--max-bytes", type=positive_int, default=65536, help="bytes to read (default 65536)")
    p.add_argument("--offset", type=non_negative_int, default=0, help="byte offset to start at")

    p = add("get", cmd_get, "download a file, or a directory as a zip")
    p.add_argument("remote")
    p.add_argument("local", nargs="?", help="local file or directory (default: current directory)")
    p.add_argument("--extract", action="store_true", help="unpack a directory instead of saving the zip")
    p.add_argument("--force", action="store_true", help="overwrite local files")

    p = add("put", cmd_put, "upload files or directories; existing remote files are kept unless --conflict says otherwise")
    p.add_argument("local", nargs="+", help="local paths, or - for stdin")
    p.add_argument("remote", help="remote file path, or directory (existing or with a trailing /)")
    p.add_argument("-r", "--recursive", action="store_true", help="upload directories")
    p.add_argument("-p", "--parents", action="store_true", help="create missing remote directories")
    p.add_argument("--conflict", choices=["reject", "skip", "autorename", "overwrite"], default="reject",
                   help="when the remote file exists (default reject)")

    p = add("mkdir", cmd_mkdir, "create a directory")
    p.add_argument("path")
    p.add_argument("-p", "--parents", action="store_true", help="create parents; succeed if it already exists")

    p = add("mv", cmd_mv, "rename or move")
    p.add_argument("source")
    p.add_argument("target")
    p.add_argument("--overwrite", action="store_true", help="replace an existing target file")

    p = add("rm", cmd_rm, "permanently delete files or directories")
    p.add_argument("path", nargs="+")
    p.add_argument("-r", "--recursive", action="store_true", help="required for any directory, even an empty one")

    p = add("url", cmd_url, "print a streaming URL for players and browsers")
    p.add_argument("path")
    p.add_argument("--with-token", action="store_true", help="embed the token in the URL (treat it as a secret)")
    return parser


def main():
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(errors="replace")
    args = build_parser().parse_args()
    base, token = parse_connection(args.url, args.token)
    try:
        args.func(Client(base, token, args.timeout), args)
    except ClientError as err:
        print(f"error: {err}", file=sys.stderr)
        return 1
    except BrokenPipeError:
        sys.stderr.close()
        return 0
    except KeyboardInterrupt:
        return 130
    return 0


if __name__ == "__main__":
    sys.exit(main())
