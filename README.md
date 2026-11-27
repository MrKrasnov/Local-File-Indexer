# Local File Indexer

A local application that scans a directory on disk and stores file metadata in SQLite. The database is the file `data/index.db`. It is created when the process starts and remains after the process stops.

Each record stores the path, name, size, permissions, modification time, directory flag, and extension.

## Run

From the project root:

```bash
go run ./cmd/api
```

The server listens on `http://localhost:8080`. Set `INDEX_DB` to use a different database file.

## Endpoints

None of the routes use a request body.

### `GET /test`

Checks that the server is responding.

```bash
curl "http://localhost:8080/test"
```

```json
{"message":"test"}
```

### `POST /scan`

Scans an absolute path and writes metadata to the database. Scanning the same path again updates existing records and removes indexed entries that are no longer on disk.

The `path` parameter is an absolute path to a file or directory.

```bash
curl -X POST --get "http://localhost:8080/scan" \
  --data-urlencode "path=/Users/mrkrasnov/Documents"
```

```json
{
  "path": "/Users/mrkrasnov/Documents",
  "saved": 42,
  "removed": 0,
  "error_count": 0,
  "errors": []
}
```

A relative path returns `400`. A missing path returns `404`.

### `GET /files`

Reads the stored metadata.

The `keyword` parameter is optional. When it is set, the response keeps records whose name or path contains that substring. Latin letters are matched regardless of case. `limit` sets the maximum number of rows, from 1 to 1000. The default is 100.

```bash
curl --get "http://localhost:8080/files" --data-urlencode "keyword=report"
```

```json
{
  "keyword": "report",
  "count": 1,
  "files": [
    {
      "path": "/Users/mrkrasnov/Documents/report.txt",
      "name": "report.txt",
      "size": 12,
      "mode": "-rw-r--r--",
      "mod_time": "2026-11-27T09:00:00Z",
      "is_dir": false,
      "extension": ".txt"
    }
  ]
}
```
