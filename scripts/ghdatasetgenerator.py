#!/usr/bin/env python3
import os
import sys
import json
import time
import datetime
from datetime import timezone
from pathlib import Path
from dotenv import load_dotenv
import requests
from git import Repo
from google.cloud import storage, bigquery
from google.oauth2 import service_account
from google.api_core.exceptions import NotFound
import shutil

# Load environment variables
BASE_DIR = Path(__file__).resolve().parent
load_dotenv("./.env")

GITHUB_TOKEN = os.getenv("GITHUB_TOKEN")
GCS_BUCKET = os.getenv("GCS_BUCKET", "ai-in-action-repo-bucket")
BQ_DATASET = os.getenv("BQ_DATASET", "my_dataset")
BQ_TABLE = os.getenv("BQ_TABLE", "repos")
PROJECT_ID = os.getenv("GCP_PROJECT_ID", "ai-in-action-461204")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")

# Initialize GCP clients, using service account file if provided
SA_KEY_PATH = os.getenv("GOOGLE_APPLICATION_CREDENTIALS")
if SA_KEY_PATH:
    # Load service account credentials and patch universe_domain for storage compatibility
    cred = service_account.Credentials.from_service_account_file(
        SA_KEY_PATH,
        scopes=["https://www.googleapis.com/auth/cloud-platform"],
    )
    cred.universe_domain = "googleapis.com"
    storage_client = storage.Client(project=PROJECT_ID, credentials=cred)
    bigquery_client = bigquery.Client(project=PROJECT_ID, credentials=cred)
else:
    storage_client = storage.Client(project=PROJECT_ID)
    bigquery_client = bigquery.Client(project=PROJECT_ID)

# GitHub API settings
GITHUB_API = "https://api.github.com"
HEADERS = {
    "Authorization": f"Bearer {GITHUB_TOKEN}",
    "User-Agent": "datasetgenerator",
    "Accept": "application/vnd.github.v3+json",
}

# Output directory
OUT_DIR = BASE_DIR / "repos"
OUT_DIR.mkdir(exist_ok=True)

# Blacklists
BLACKLIST_KEYWORDS = {
    "awesome","roadmap","guide","handbook","resources","list","curated",
    "cheatsheet","interview","books","how-to","howto","beginners","best","33"
}
BLACKLIST_REPOS = {
    "cs-self-learning","hello-algo","HelloGitHub","learn-regex",
    "javascript-algorithms","leetcode","leetcode-master","hiring-without-whiteboards"
}
GOOD_LICENSES = {"MIT","Apache-2.0","BSD-3-Clause","GPL-3.0","LGPL-3.0"}

def should_include(repo):
    # Basic filters
    if repo.get("archived") or repo.get("fork"): return False
    if repo.get("open_issues_count",0) < 10: return False
    # License check
    lic = repo.get("license") or {}
    if lic.get("spdx_id") not in GOOD_LICENSES: return False
    # Push/Issue recency
    cutoff = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=30)
    pushed_dt = datetime.datetime.fromisoformat(repo["pushed_at"][:-1]).replace(tzinfo=datetime.timezone.utc)
    updated_dt = datetime.datetime.fromisoformat(repo["updated_at"][:-1]).replace(tzinfo=datetime.timezone.utc)
    if pushed_dt < cutoff and updated_dt < cutoff:
        return False
    # Keyword blacklist
    name = repo.get("name") or ""
    desc = repo.get("description") or ""
    name_desc = f"{name} {desc}".lower()
    if any(k in name_desc for k in BLACKLIST_KEYWORDS): return False
    if repo.get("name") in BLACKLIST_REPOS: return False
    # Contributor count
    since = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=365)).isoformat()+"Z"
    url = f"{GITHUB_API}/repos/{repo['full_name']}/contributors?per_page=100&since={since}"
    r = requests.get(url, headers=HEADERS)
    if not r.ok or len(r.json()) < 20: return False
    return True

def clone_and_save(repo):
    name = repo["name"]
    repo_dir = OUT_DIR / name
    meta_path = repo_dir / "metadata.json"
    if repo_dir.exists() and meta_path.exists():
        return json.loads(meta_path.read_text())
    # Clone
    Repo.clone_from(repo["clone_url"], str(repo_dir), depth=1)
    # Fetch languages
    lang = {}
    r = requests.get(f"{GITHUB_API}/repos/{repo['full_name']}/languages", headers=HEADERS)
    if r.ok: lang = r.json()
    # Fetch README raw
    readme = ""
    r = requests.get(f"{GITHUB_API}/repos/{repo['full_name']}/readme",
                     headers={**HEADERS, "Accept":"application/vnd.github.v3.raw"})
    if r.ok: readme = r.text
    # Compute score and reasons
    pushed_dt = datetime.datetime.fromisoformat(repo["pushed_at"][:-1]).replace(tzinfo=datetime.timezone.utc)
    score = min(1.0, repo["stargazers_count"]/1000 + repo["open_issues_count"]/100 +
                (0.5 if pushed_dt > datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=14) else 0))
    reasons = []
    if repo["stargazers_count"]>500: reasons.append("High stars")
    if repo["open_issues_count"]>10: reasons.append("Active issues")
    if pushed_dt > datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=14):
        reasons.append("Recently updated")
    meta = {
        "name": repo["name"],
        "full_name": repo["full_name"],
        "owner": repo["owner"]["login"],
        "html_url": repo["html_url"],
        "description": repo.get("description"),
        "language": repo.get("language"),
        "stargazers_count": repo.get("stargazers_count"),
        "watchers_count": repo.get("watchers_count"),
        "forks_count": repo.get("forks_count"),
        "open_issues_count": repo.get("open_issues_count"),
        "license": repo.get("license",{}).get("spdx_id"),
        "homepage": repo.get("homepage"),
        "image_url": repo["owner"]["avatar_url"],
        "default_branch": repo.get("default_branch"),
        "created_at": repo.get("created_at"),
        "pushed_at": repo.get("pushed_at"),
        "size": repo.get("size"),
        "visibility": repo.get("visibility"),
        "archived": repo.get("archived"),
        "allow_forking": repo.get("allow_forking"),
        "is_template": repo.get("is_template"),
        "topics": repo.get("topics",[]),
        "languages": list(lang.keys()),
        "readme": readme,
        "score": score,
        "relevance_reason": ", ".join(reasons)
    }
    repo_dir.mkdir(exist_ok=True)
    meta_path.write_text(json.dumps(meta, indent=2))
    return meta

def main():
    desired = 3
    # Clear out the repos directory on each run
    if OUT_DIR.exists():
        for d in OUT_DIR.iterdir():
            if d.is_dir():
                shutil.rmtree(d)
    # First, load any already-cloned repos' metadata
    selected = []
    if OUT_DIR.exists():
        for d in OUT_DIR.iterdir():
            meta_path = d / "metadata.json"
            if meta_path.exists():
                try:
                    selected.append(json.loads(meta_path.read_text()))
                except Exception as e:
                    print(f"Failed to load metadata for {d.name}: {e}")
    # If no local repos, perform GitHub search & clone
    if not selected:
        seen = set()
        page = 1
        per_page = 100
        while len(selected) < desired:
            q = "stars:>100"
            r = requests.get(
                f"{GITHUB_API}/search/repositories",
                params={"q": q, "sort": "stars", "order": "desc", "per_page": per_page, "page": page},
                headers=HEADERS,
            )
            if not r.ok:
                print(f"GitHub search failed: {r.status_code}")
                break
            items = r.json().get("items", [])
            for repo in items:
                full = repo["full_name"]
                if full in seen:
                    continue
                seen.add(full)
                if should_include(repo):
                    meta = clone_and_save(repo)
                    if meta:
                        selected.append(meta)
                        if len(selected) >= desired:
                            break
            page += 1
    else:
        print(f"Loaded {len(selected)} repos from local directory")
    # Trim to desired count
    selected = selected[:desired]
    # Cleanup
    for d in OUT_DIR.iterdir():
        if d.is_dir() and d.name not in {m["name"] for m in selected}:
            for p in d.rglob("*"):
                p.unlink()
            d.rmdir()
    # Write repos.json as NDJSON
    out_json = BASE_DIR / "repos.json"
    with open(out_json, "w", encoding="utf-8") as f:
        for repo in selected:
            f.write(json.dumps(repo) + "\n")
    print(f"Wrote {len(selected)} repos (NDJSON) to {out_json}")
    if not selected:
        print("No repositories selected; skipping GCS upload and BigQuery load.")
        return
    # Upload to GCS
    gcs_src = str(out_json)
    gcs_dest = "input/repos.json"
    print(f"Uploading to gs://{GCS_BUCKET}/{gcs_dest}")
    bucket = storage_client.bucket(GCS_BUCKET)
    if not bucket.exists():
        print(f"Bucket {GCS_BUCKET} does not exist. Creating it...")
        bucket = storage_client.create_bucket(GCS_BUCKET, location=LOCATION)
    # Clear existing objects under the input/ prefix
    print(f"Clearing existing objects in gs://{GCS_BUCKET}/input/")
    blobs = list(bucket.list_blobs(prefix="input/"))
    if blobs:
        bucket.delete_blobs(blobs)
        print(f"Deleted {len(blobs)} objects.")
    blob = bucket.blob(gcs_dest)
    blob.upload_from_filename(gcs_src)

    # Upload cloned repository files to GCS via Python client
    print("Uploading cloned repository files to GCS...")
    files_uploaded = 0
    for root, _, files in os.walk(OUT_DIR):
        for fname in files:
            local_path = os.path.join(root, fname)
            rel_path = os.path.relpath(local_path, OUT_DIR)
            gcs_repo_dest = f"input/repos/{rel_path}"
            print(f"Uploading {local_path} to gs://{GCS_BUCKET}/{gcs_repo_dest}")
            repo_blob = bucket.blob(gcs_repo_dest)
            repo_blob.upload_from_filename(local_path)
            files_uploaded += 1
    print(f"Uploaded {files_uploaded} cloned repository files to GCS.")
    # Ensure BigQuery dataset exists
    dataset_ref = bigquery_client.dataset(BQ_DATASET)
    try:
        bigquery_client.get_dataset(dataset_ref)
    except NotFound:
        print(f"Dataset {BQ_DATASET} not found. Creating it...")
        dataset = bigquery.Dataset(dataset_ref)
        dataset.location = LOCATION
        bigquery_client.create_dataset(dataset, exists_ok=True)
    # Load into BigQuery
    print(f"Loading into BigQuery {BQ_DATASET}.{BQ_TABLE}")
    table_ref = bigquery_client.dataset(BQ_DATASET).table(BQ_TABLE)
    job = bigquery_client.load_table_from_uri(
        f"gs://{GCS_BUCKET}/{gcs_dest}", table_ref,
        job_config=bigquery.LoadJobConfig(source_format=bigquery.SourceFormat.NEWLINE_DELIMITED_JSON, autodetect=True)
    )
    job.result()
    print("BigQuery load complete:", job.output_rows)

if __name__=="__main__":
    main()