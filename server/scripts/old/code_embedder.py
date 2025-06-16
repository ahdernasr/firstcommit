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
import tiktoken
from google.cloud import storage
# GCS settings from environment
GCS_BUCKET = os.getenv("GCS_BUCKET")
GCS_INPUT_PREFIX = os.getenv("GCS_INPUT_PREFIX", "input")

tokenizer = tiktoken.get_encoding("cl100k_base")

def chunk_text_by_token_limit(text, token_limit=2048):
    tokens = tokenizer.encode(text)
    chunks = [tokens[i:i+token_limit] for i in range(0, len(tokens), token_limit)]
    return [tokenizer.decode(chunk) for chunk in chunks]

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

# Ensure GCS_BUCKET is set
if not GCS_BUCKET:
    raise ValueError("GCS_BUCKET environment variable must be set to your bucket name")

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

def embed_chunks(repo_name: str, chunk_data: list):
    """
    Batch up to 250 chunks for text-embedding-005 and insert into MongoDB.
    """
    endpoint = (
        f"https://{LOCATION}-aiplatform.googleapis.com/v1/"
        f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/text-embedding-005:predict"
    )
    headers = {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json; charset=utf-8"
    }

    import random
    BATCH_SIZE = 250

    def send_batch(repo_name, batch):
        instances = [
            {"task_type": "RETRIEVAL_DOCUMENT", "title": title, "content": chunk}
            for title, chunk in batch
        ]
        payload = {"instances": instances}

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
                # For other errors, break to handle below
                break
        else:
            print(f"Failed after {max_retries} retries due to repeated 429s or other errors.")
            return

        if response.status_code == 400 and "input token count" in response.text.lower():
            if len(batch) > 1:
                mid = len(batch) // 2
                first_half = batch[:mid]
                second_half = batch[mid:]
                send_batch(repo_name, first_half)
                send_batch(repo_name, second_half)
                return
            else:
                title = batch[0][0]
                print(f"Skipping chunk {title} due to input token count limit exceeded.")
                return
        elif response.status_code != 200:
            print(f"Request failed with status {response.status_code}: {response.text}")
            response.raise_for_status()
            return

        result = response.json()
        predictions = result.get("predictions", [])
        if len(predictions) != len(batch):
            print("Mismatch between predictions and input chunks.")
            return

        for (title, chunk), prediction in zip(batch, predictions):
            chunk_number = int(title.rsplit("::chunk_", 1)[-1]) if "::chunk_" in title else None
            embeddings = prediction["embeddings"]["values"]
            doc = {
                "repo": repo_name,
                "path": title.split("::chunk_", 1)[0],
                "chunk": chunk_number,
                "embedding": embeddings
            }
            db[repo_name].insert_one(doc)
            print(f"Inserted embedding for {repo_name}/{title}")

    batches = []
    current_batch = []
    current_tokens = 0

    for title, chunk in chunk_data:
        token_count = len(tokenizer.encode(chunk))
        if token_count > 2048:
            print(f"Skipping chunk {title} with {token_count} tokens (over per-instance limit)")
            continue
        if token_count > 20000:
            print(f"Skipping chunk {title} with {token_count} tokens (over batch limit)")
            continue
        if current_tokens + token_count > 15000:
            if current_batch:
                batches.append(current_batch)
            current_batch = [(title, chunk)]
            current_tokens = token_count
        else:
            current_batch.append((title, chunk))
            current_tokens += token_count

    if current_batch:
        batches.append(current_batch)
        current_tokens = 0

    for batch in batches:
        send_batch(repo_name, batch)

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

    # Gather all chunks across files
    chunk_data = []
    for root, dirs, files in os.walk(repo_dir):
        dirs[:] = [d for d in dirs if d not in GENERATED_DIRS and d != '.git']
        for fname in files:
            ext = os.path.splitext(fname)[1].lower()
            if ext not in ALLOWED_EXTENSIONS and fname not in SPECIAL_FILENAMES:
                continue
            rel_dir = os.path.relpath(root, repo_dir)
            rel_path = os.path.normpath(os.path.join(rel_dir, fname)) if rel_dir != '.' else fname
            if spec.match_file(rel_path):
                continue
            file_path = os.path.join(root, fname)
            try:
                with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                chunks = chunk_text_by_token_limit(content, token_limit=2048)
                if ext == ".sql":
                    chunks = chunks[:SQL_CHUNK_LIMIT]
                chunks = chunks[:MAX_CHUNKS]
                for i, chunk in enumerate(chunks):
                    title = f"{rel_path}::chunk_{i+1}" if len(chunks) > 1 else rel_path
                    chunk_data.append((title, chunk))
            except Exception as e:
                print(f"Skipping file {rel_path} due to error: {e}")

    # Dump chunks to JSONL and upload to GCS for batch embedding
    import json, tempfile
    # Write local JSONL file
    tmpfile = tempfile.NamedTemporaryFile(mode="w", delete=False, suffix=".jsonl")
    for title, chunk in chunk_data:
        json.dump({"text": chunk, "key": title}, tmpfile)
        tmpfile.write("\n")
    tmpfile.close()
    # Upload to GCS
    storage_client = storage.Client(credentials=credentials, project=PROJECT_ID)
    bucket = storage_client.bucket(GCS_BUCKET)
    blob = bucket.blob(f"{GCS_INPUT_PREFIX}/{repo_name}_chunks.jsonl")
    blob.upload_from_filename(tmpfile.name)
    print(f"Uploaded chunks JSONL to gs://{GCS_BUCKET}/{GCS_INPUT_PREFIX}/{repo_name}_chunks.jsonl")

    start_time = time.time()
    embed_chunks(repo_name, chunk_data)
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