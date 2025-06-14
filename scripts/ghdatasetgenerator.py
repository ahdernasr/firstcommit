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
from requests.exceptions import ReadTimeout
import shutil
from concurrent.futures import ThreadPoolExecutor

# Directories to skip (generated or vendored)
GENERATED_DIRS = {"vendor", "node_modules", "third_party", "build", "dist", "target"}

# Allowed file extensions and special filenames
ALLOWED_EXTENSIONS = {
    # Common scripting & compiled languages
    ".py", ".js", ".ts", ".go", ".java", ".kt", ".swift",
    ".cpp", ".c", ".h", ".hpp", ".cc", ".cxx", ".mm",
    ".rs", ".cs", ".rb", ".php", ".dart", ".lua", ".scala",
    ".hs", ".erl", ".ex", ".exs", ".r", ".jl", ".m", ".mm",
    ".kt", ".groovy", ".clj", ".cljc", ".cljx",
    ".cr", ".vb", ".vbs", ".fs", ".fsx", ".ml",
    ".mli", ".adb", ".ads", ".pas", ".f", ".f90", ".f95",
    ".asm", ".s", ".ps1", ".bat", ".cmd", ".sh", ".zsh", ".fish",
    ".sql", ".psql",

    # Markup & config
    ".json", ".yaml", ".yml", ".toml", ".xml", ".md", ".html", ".htm"
}

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
    # Patch universe_domain for storage compatibility
    # set private attribute since the property has no setter
    cred._universe_domain = "googleapis.com"
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
    "cs-self-learning", "hello-algo", "HelloGitHub", "learn-regex",
    "javascript-algorithms", "leetcode", "leetcode-master",
    "hiring-without-whiteboards", "freecodecamp"
}
# Ensure blacklist is case-insensitive
BLACKLIST_REPOS_LOWER = {r.lower() for r in BLACKLIST_REPOS}
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
    # Case-insensitive repo name blacklist
    name_lower = repo.get("name", "").lower()
    if name_lower in BLACKLIST_REPOS_LOWER:
        return False
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
    desired = 2
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
    # Write repos.json as a proper JSON array
    out_json = BASE_DIR / "repos.json"
    with open(out_json, "w", encoding="utf-8") as f:
        json.dump(selected, f, indent=2, ensure_ascii=False)
        f.write("\n")
    print(f"Wrote {len(selected)} repos to {out_json}")
    # Also write NDJSON for BigQuery load
    ndjson_path = BASE_DIR / "repos_ndjson.jsonl"
    with open(ndjson_path, "w", encoding="utf-8") as nd_f:
        for repo in selected:
            nd_f.write(json.dumps(repo) + "\n")
    print(f"Wrote {len(selected)} repos to {ndjson_path} (NDJSON)")
    if not selected:
        print("No repositories selected; skipping GCS upload and BigQuery load.")
        return
    # Upload to GCS
    # Use NDJSON for BigQuery
    gcs_src = str(ndjson_path)
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
        print(f"Deleting {len(blobs)} objects in parallel...")
        with ThreadPoolExecutor(max_workers=20) as executor:
            list(executor.map(lambda blob: blob.delete(), blobs))
        print(f"Deleted {len(blobs)} objects.")
    blob = bucket.blob(gcs_dest)
    blob.upload_from_filename(gcs_src)

    # Upload cloned repository files to GCS in parallel
    print("Uploading cloned repository files to GCS in parallel…")
    tasks = []
    for root, dirs, files in os.walk(OUT_DIR):
        # Skip generated or vendored directories
        dirs[:] = [d for d in dirs if d not in GENERATED_DIRS]
        for fname in files:
            ext = os.path.splitext(fname)[1].lower()
            if ext not in ALLOWED_EXTENSIONS:
                continue
            local_path = os.path.join(root, fname)
            rel_path   = os.path.relpath(local_path, OUT_DIR)
            code_dest  = f"input/repos/{rel_path}"
            tasks.append((local_path, code_dest))
    def upload_task(args):
        local_path, dest = args
        blob = bucket.blob(dest)
        print(f"Uploading {local_path} to gs://{GCS_BUCKET}/{dest}")
        for attempt in range(3):
            try:
                blob.upload_from_filename(local_path, timeout=300)
                print(f"✔ Uploaded {local_path}")
                return True
            except ReadTimeout:
                print(f"⏱ Timeout uploading {local_path}, retrying {attempt+1}/3…")
                time.sleep(2 ** attempt)
        print(f"❌ Failed to upload {local_path} after 3 retries")
        return False
    success = 0
    with ThreadPoolExecutor(max_workers=20) as pool:
        for ok in pool.map(upload_task, tasks):
            if ok:
                success += 1
    print(f"Uploaded {success}/{len(tasks)} files.")
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