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
from git.exc import GitCommandError
from google.cloud import storage, bigquery
from google.oauth2 import service_account
from google.api_core.exceptions import NotFound
from requests.exceptions import ReadTimeout
import shutil
from concurrent.futures import ThreadPoolExecutor
import logging
import argparse

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

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
log = logging.getLogger(__name__)

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
    "cheatsheet","interview","books","how-to","howto","beginners","best","33", "fuck"
}
BLACKLIST_REPOS = {
    "cs-self-learning", "hello-algo", "HelloGitHub", "learn-regex",
    "javascript-algorithms", "leetcode", "leetcode-master",
    "hiring-without-whiteboards", "freecodecamp", "computer-science"
}
# Ensure blacklist is case-insensitive
BLACKLIST_REPOS_LOWER = {r.lower() for r in BLACKLIST_REPOS}
GOOD_LICENSES = {"MIT","Apache-2.0","BSD-3-Clause","GPL-3.0","LGPL-3.0"}

DEFAULT_DESIRED = int(os.getenv("DESIRED_REPOS", "25"))

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
    # If a previous attempt left a partial dir (no metadata), wipe it
    if repo_dir.exists() and not meta_path.exists():
        log.info("Removing stale directory %s from previous failed clone…", repo_dir)
        shutil.rmtree(repo_dir, ignore_errors=True)
    # Clone
    try:
        # First attempt: normal shallow clone
        Repo.clone_from(repo["clone_url"], str(repo_dir), depth=1)
    except GitCommandError as e:
        # Common failure when git‑lfs is not installed: retry with LFS disabled
        if "git-lfs" in str(e) or "filter-process" in str(e):
            log.info("git‑lfs not available for %s; retrying without LFS...", repo['full_name'])
            os.environ["GIT_LFS_SKIP_SMUDGE"] = "1"
            try:
                Repo.clone_from(repo["clone_url"], str(repo_dir), depth=1)
            except GitCommandError as e2:
                log.warning("Clone still failed for %s: %s. Skipping.", repo['full_name'], e2)
                return None
            finally:
                os.environ.pop("GIT_LFS_SKIP_SMUDGE", None)
        else:
            log.warning("Clone failed for %s: %s. Skipping.", repo['full_name'], e)
            return None
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


# ── Parallel clone helper ──
def clone_repos_parallel(repos, max_workers: int = 8):
    """Clone repos concurrently and return list of metadata dicts."""
    if not repos:
        return []
    log.info("Cloning %d repos in parallel (max_workers=%d)…", len(repos), max_workers)
    metas = []
    with ThreadPoolExecutor(max_workers=max_workers) as pool:
        for meta in pool.map(clone_and_save, repos):
            if meta:
                metas.append(meta)
    return metas

def main(desired: int):
    overall_start = time.perf_counter()
    # Clear out the repos directory on each run
    if OUT_DIR.exists():
        for d in OUT_DIR.iterdir():
            if d.is_dir():
                shutil.rmtree(d)
    selected = []
    seen = set()
    page = 1
    per_page = 100
    repos_to_clone = []
    while len(selected) + len(repos_to_clone) < desired:
        q = "stars:>100"
        r = requests.get(
            f"{GITHUB_API}/search/repositories",
            params={
                "q": q,
                "sort": "stars",
                "order": "desc",
                "per_page": per_page,
                "page": page,
            },
            headers=HEADERS,
        )
        if not r.ok:
            log.warning("GitHub search failed: %s", r.status_code)
            break
        items = r.json().get("items", [])
        if not items:
            break  # no more results

        for repo in items:
            full = repo["full_name"]
            if full in seen:
                continue
            seen.add(full)
            if should_include(repo):
                repos_to_clone.append(repo)
                if len(selected) + len(repos_to_clone) >= desired:
                    break
        page += 1

    # ── clone the batch concurrently ──
    selected.extend(clone_repos_parallel(repos_to_clone))
    # Trim to desired count
    selected = selected[:desired]
    # Cleanup
    for d in OUT_DIR.iterdir():
        if d.is_dir() and d.name not in {m["name"] for m in selected}:
            log.info("Removing obsolete repo directory %s…", d)
            shutil.rmtree(d, ignore_errors=True)
    # Write repos.json as a proper JSON array
    out_json = BASE_DIR / "repos.json"
    with open(out_json, "w", encoding="utf-8") as f:
        json.dump(selected, f, indent=2, ensure_ascii=False)
        f.write("\n")
    # Also write NDJSON for BigQuery load
    ndjson_path = BASE_DIR / "repos_ndjson.jsonl"
    with open(ndjson_path, "w", encoding="utf-8") as nd_f:
        for repo in selected:
            nd_f.write(json.dumps(repo) + "\n")
    if not selected:
        elapsed = time.perf_counter() - overall_start
        log.info("Total runtime: %.2f s", elapsed)
        return
    # Upload to GCS
    # Use NDJSON for BigQuery
    gcs_src = str(ndjson_path)
    gcs_dest = "input/repos.json"
    log.info("Uploading to gs://%s/%s", GCS_BUCKET, gcs_dest)
    bucket = storage_client.bucket(GCS_BUCKET)
    if not bucket.exists():
        log.info("Bucket %s does not exist. Creating it...", GCS_BUCKET)
        bucket = storage_client.create_bucket(GCS_BUCKET, location=LOCATION)
    # Clear existing objects under the input/ prefix
    log.info("Clearing existing objects in gs://%s/input/", GCS_BUCKET)
    blobs = list(bucket.list_blobs(prefix="input/"))
    if blobs:
        log.info("Deleting %d objects in parallel...", len(blobs))
        with ThreadPoolExecutor(max_workers=20) as executor:
            list(executor.map(lambda blob: blob.delete(), blobs))
        log.info("Deleted %d objects.", len(blobs))
    blob = bucket.blob(gcs_dest)
    blob.upload_from_filename(gcs_src)

    # Upload cloned repository files to GCS in parallel
    log.info("Uploading cloned repository files to GCS in parallel…")
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
        log.debug("Uploading %s", local_path)

        for attempt in range(4):
            try:
                blob.upload_from_filename(local_path, timeout=300)
                log.debug("✔ Uploaded %s", local_path)
                return True
            except (ReadTimeout, requests.exceptions.RequestException,
                    ConnectionResetError, Exception) as exc:
                # For unexpected exceptions, log and retry a few times
                log.warning("Upload failed for %s (attempt %d/4): %s",
                            local_path, attempt + 1, exc)
                time.sleep(2 ** attempt)
        log.error("Failed to upload %s after 4 attempts", local_path)
        return False
    success = 0
    try:
        with ThreadPoolExecutor(max_workers=10) as pool:
            for ok in pool.map(upload_task, tasks):
                if ok:
                    success += 1
    except Exception as exc:
        log.error("Unexpected thread pool error during uploads: %s", exc)
    log.info("Uploaded %d/%d files.", success, len(tasks))
    # Ensure BigQuery dataset exists
    dataset_ref = bigquery_client.dataset(BQ_DATASET)
    try:
        bigquery_client.get_dataset(dataset_ref)
    except NotFound:
        log.info("Dataset %s not found. Creating it...", BQ_DATASET)
        dataset = bigquery.Dataset(dataset_ref)
        dataset.location = LOCATION
        bigquery_client.create_dataset(dataset, exists_ok=True)
    # Load into BigQuery
    log.info("Loading into BigQuery %s.%s", BQ_DATASET, BQ_TABLE)
    table_ref = bigquery_client.dataset(BQ_DATASET).table(BQ_TABLE)
    job = bigquery_client.load_table_from_uri(
        f"gs://{GCS_BUCKET}/{gcs_dest}", table_ref,
        job_config=bigquery.LoadJobConfig(source_format=bigquery.SourceFormat.NEWLINE_DELIMITED_JSON, autodetect=True)
    )
    job.result()
    elapsed = time.perf_counter() - overall_start
    log.info("BigQuery load complete: %s rows", job.output_rows)
    log.info("Total runtime: %.2f s", elapsed)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="GitHub dataset generator")
    parser.add_argument(
        "-n",
        "--desired",
        type=int,
        default=DEFAULT_DESIRED,
        help="Number of repositories to fetch (default: %(default)s)",
    )
    args = parser.parse_args()
    main(args.desired)