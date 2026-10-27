"""Add TMDB poster and backdrop URLs to metadata.csv, using the official TMDB API.

Usage (the token is read from the environment, never stored in the repo):

    export TMDB_TOKEN="<your TMDB API Read Access Token or v3 API key>"
    python3 tmdb_enrich.py metadata.csv metadata.csv

Get a free key at https://www.themoviedb.org/settings/api. Both the long
"API Read Access Token" and the 32-character v3 "API Key" work.
This product uses the TMDB API but is not endorsed or certified by TMDB.
"""

import csv
import json
import os
import sys
import time
import urllib.parse
import urllib.request

API = "https://api.themoviedb.org/3"
IMG = "https://image.tmdb.org/t/p"


def search(kind, title, year, token):
    params = {"query": title, "language": "en-US", "include_adult": "false"}
    if year:
        params["year" if kind == "movie" else "first_air_date_year"] = year
    headers = {"Accept": "application/json"}
    if len(token) == 32:  # v3 API key goes in the query string
        params["api_key"] = token
    else:  # Read Access Token goes in the Authorization header
        headers["Authorization"] = f"Bearer {token}"
    url = f"{API}/search/{kind}?{urllib.parse.urlencode(params)}"
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=20) as resp:
        results = json.load(resp).get("results", [])
    return results[0] if results else None


def main(src, dst):
    token = os.environ.get("TMDB_TOKEN")
    if not token:
        sys.exit("Set TMDB_TOKEN first (API Read Access Token or v3 API key from https://www.themoviedb.org/settings/api).")

    with open(src, encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    extra = ["tmdb_url", "poster_url", "backdrop_url"]
    fields = list(rows[0].keys()) + [c for c in extra if c not in rows[0]]

    missing = 0
    for row in rows:
        for c in extra:
            row.setdefault(c, "")
        if row["poster"] != "yes":
            continue  # deliberately broken rows (home video, unidentified file) stay without artwork
        kind = "movie" if row["type"] == "movie" else "tv"
        hit = search(kind, row["title"], row["year"], token)
        if not hit:
            missing += 1
            print(f"not found: {row['title']}", file=sys.stderr)
            continue
        row["tmdb_url"] = f"https://www.themoviedb.org/{kind}/{hit['id']}"
        row["poster_url"] = f"{IMG}/w500{hit['poster_path']}" if hit.get("poster_path") else ""
        row["backdrop_url"] = f"{IMG}/w1280{hit['backdrop_path']}" if hit.get("backdrop_path") else ""
        time.sleep(0.05)  # well under TMDB's rate limit

    with open(dst, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)
    print(f"done: {len(rows)} rows, {missing} not found", file=sys.stderr)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
