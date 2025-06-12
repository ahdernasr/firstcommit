#!/usr/bin/env python3

import os
import json
from dotenv import load_dotenv
import requests
from google.oauth2 import service_account
from google.auth.transport.requests import Request
from pymongo import MongoClient
import certifi
from pathspec import PathSpec
import concurrent.futures
import time

import os

# Chunk size in characters
CHUNK_SIZE = 50000

# Maximum chunks to embed for SQL files
SQL_CHUNK_LIMIT = 1

# Maximum chunks to embed for any file
MAX_CHUNKS = 10

# Directories to skip (generated or vendored)
GENERATED_DIRS = {"vendor", "node_modules", "third_party", "build", "dist", "target"}

# Allowed file extensions and special filenames
ALLOWED_EXTENSIONS = {
    # Common scripting & compiled languages
    ".py", ".js", ".ts", ".go", ".java", ".kt", ".swift",
    ".cpp", ".c", ".h", ".hpp", ".cc", ".cxx", ".mm",
    ".rs", ".cs", ".rb", ".php", ".dart", ".lua", ".scala",
    ".hs", ".erl", ".ex", ".exs", ".r", ".jl", ".m", ".mm",
    ".kt", ".groovy", ".groovy", ".clj", ".cljc", ".cljx", 
    ".cr", ".swift", ".vb", ".vbs", ".fs", ".fsx", ".ml", 
    ".mli", ".adb", ".ads", ".pas", ".f", ".f90", ".f95",
    ".asm", ".s", ".ps1", ".bat", ".cmd", ".sh", ".zsh", ".fish",
    ".sql", ".psql",

    # Markup & config (if you want to search these too)
    ".json", ".yaml", ".yml", ".toml", ".xml", ".md", ".html", ".htm"
}
SPECIAL_FILENAMES = {"Dockerfile"}

# Load environment variables from .env
load_dotenv(dotenv_path=os.path.join(os.path.dirname(__file__), '.env'))

# Configuration from environment
PROJECT_ID = os.getenv("GCP_PROJECT_ID")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")
MODEL = os.getenv("EMBEDDING_MODEL")
KEYFILE = os.getenv("GCP_KEYFILE_PATH")
MONGODB_URI = os.getenv("MONGODB_URI")
DB_NAME = os.getenv("DB_NAME", "vector_db")
COLLECTION_NAME = os.getenv("COLLECTION_NAME", "files")

# Authenticate with service account
credentials = service_account.Credentials.from_service_account_file(
    KEYFILE,
    scopes=["https://www.googleapis.com/auth/cloud-platform"]
)
auth_request = Request()
credentials.refresh(auth_request)
access_token = credentials.token

# Initialize MongoDB client
mongo_client = MongoClient(MONGODB_URI, tlsCAFile=certifi.where())
db = mongo_client[DB_NAME]

def load_gitignore(repo_path: str) -> PathSpec:
    """
    Load .gitignore patterns from repo_path and return a PathSpec matcher.
    """
    ignore_file = os.path.join(repo_path, '.gitignore')
    if os.path.exists(ignore_file):
        with open(ignore_file, 'r') as f:
            lines = f.readlines()
        return PathSpec.from_lines('gitwildmatch', lines)
    return PathSpec.from_lines('gitwildmatch', [])

def embed_file(repo_name: str, file_path: str, rel_path: str):
    """
    Read file, generate embedding, and insert into MongoDB.
    """
    with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
        content = f.read()

    # Split into chunks
    chunks = [content[i:i+CHUNK_SIZE] for i in range(0, len(content), CHUNK_SIZE)]

    # If SQL file, only embed the first chunk
    ext = os.path.splitext(rel_path)[1].lower()
    if ext == ".sql":
        chunks = chunks[:SQL_CHUNK_LIMIT]
    # Enforce a global chunk limit
    chunks = chunks[:MAX_CHUNKS]

    for idx, chunk in enumerate(chunks, start=1):
        title = f"{rel_path}::chunk_{idx}" if len(chunks) > 1 else rel_path

        endpoint = (
            f"https://{LOCATION}-aiplatform.googleapis.com/v1/"
            f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/"
            f"models/{MODEL}:predict"
        )
        payload = {
            "instances": [
                {"task_type": "RETRIEVAL_DOCUMENT", "title": title, "content": chunk}
            ]
        }
        headers = {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json; charset=utf-8"
        }
        import random

        max_retries = 5
        for attempt in range(max_retries):
            response = requests.post(endpoint, headers=headers, json=payload)
            if response.status_code == 200:
                break
            elif response.status_code == 429:
                retry_after = int(response.headers.get("Retry-After", "5"))
                jitter = random.uniform(0, 1.5)
                wait = retry_after + jitter * (2 ** attempt)
                print(f"429 received. Waiting {wait:.1f}s before retrying (attempt {attempt + 1})...")
                time.sleep(wait)
            else:
                print(f"Request failed with status {response.status_code}: {response.text}")
                response.raise_for_status()
        else:
            print(f"Failed after {max_retries} retries due to repeated 429s or other errors.")
            return
        result = response.json()
        embeddings = result["predictions"][0]["embeddings"]["values"]

        doc = {
            "repo": repo_name,
            "path": rel_path,
            "chunk": idx if len(chunks) > 1 else None,
            "embedding": embeddings
        }
        db[repo_name].insert_one(doc)
        print(f"Inserted embedding for {repo_name}/{title}")
    # No per-file timing or return value

def process_repo(repo_dir: str):
    """
    Delete existing embeddings for this repo and embed all files.
    """
    repo_name = os.path.basename(repo_dir)
    print(f"\nProcessing repo: {repo_name}")
    # Drop the repository-specific collection to clear old embeddings
    db.drop_collection(repo_name)

    # Load ignore patterns
    spec = load_gitignore(repo_dir)

    # Collect files to embed
    tasks = []
    for root, dirs, files in os.walk(repo_dir):
        # Skip .git and generated directories
        dirs[:] = [d for d in dirs if d not in GENERATED_DIRS and d != '.git']

        for fname in files:
            ext = os.path.splitext(fname)[1].lower()
            # Check allowed extensions or special filename
            if ext not in ALLOWED_EXTENSIONS and fname not in SPECIAL_FILENAMES:
                continue

            rel_dir = os.path.relpath(root, repo_dir)
            rel_path = os.path.normpath(os.path.join(rel_dir, fname)) if rel_dir != '.' else fname
            if spec.match_file(rel_path):
                continue

            file_path = os.path.join(root, fname)
            tasks.append((repo_name, file_path, rel_path))

    # Run embeddings in parallel
    start_time = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        futures = [executor.submit(embed_file, *args) for args in tasks]
        for f in concurrent.futures.as_completed(futures):
            try:
                f.result()
            except Exception as e:
                print(f"Embedding task failed: {e}")
    elapsed = time.time() - start_time
    return elapsed

if __name__ == "__main__":
    total_elapsed = 0.0
    base_dir = os.path.normpath(os.path.join(
        os.path.dirname(__file__),
        "..", "..",
        "FirstCommit", "dataset", "repos"
    ))
    repos = [d for d in os.listdir(base_dir) if os.path.isdir(os.path.join(base_dir, d))]
    total = len(repos)

    for idx, repo in enumerate(repos, start=1):
        print(f"\n=== Processing repo {idx}/{total}: {repo} ===")
        repo_elapsed = process_repo(os.path.join(base_dir, repo))
        minutes = repo_elapsed
        minutes /= 60
        hours = minutes / 60
        print(f"Repository embedding time: {repo_elapsed:.0f}s ({minutes:.2f} minutes)")
        total_elapsed += repo_elapsed
        total_minutes = total_elapsed / 60
        total_hours = total_minutes / 60
        print(f"Total embedding time: {total_minutes:.0f} minutes ({total_hours:.2f} hours)")