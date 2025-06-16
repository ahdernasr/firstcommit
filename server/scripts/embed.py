#!/usr/bin/env python3
import os
import sys
import json
from pathlib import Path
from dotenv import load_dotenv
from google.cloud import storage
import google.cloud.aiplatform as aiplatform
from vertexai.language_models import TextEmbeddingModel
from pymongo import MongoClient
from pymongo import ReplaceOne, WriteConcern
import certifi
import time
# Local embedding support
from sentence_transformers import SentenceTransformer
from concurrent.futures import ThreadPoolExecutor

# Load environment
BASE_DIR = Path(__file__).resolve().parent
sys.path.append(str(BASE_DIR))  # so local modules in scripts/ can be imported
load_dotenv("./.env")

# MongoDB settings
# Set MONGO_DB_NAME in your .env file if you want a different database name.
MONGO_URI = os.getenv("MONGODB_URI")
FEDERATED_MONGO_URI = os.getenv("FEDERATED_MONGODB_URI")

# Validate MongoDB URIs
if not MONGO_URI or "localhost" in MONGO_URI:
    print("Error: MONGODB_CONNECTION_STRING is not set or is pointing to localhost. Please set MONGODB_CONNECTION_STRING in your .env to your Atlas MongoDB URI.")
    sys.exit(1)
if not FEDERATED_MONGO_URI:
    print("Error: FEDERATED_MONGODB_URI is not set. Please set the Atlas Data Federation connection string in your .env.")
    sys.exit(1)

# GCP settings
PROJECT_ID = os.getenv("GCP_PROJECT_ID")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")
GCS_BUCKET = os.getenv("GCS_BUCKET")

# Vertex AI settings

# --- embedding parameters ----------------------------------------------------
MAX_CODE_TOKENS = 2048  # keep chunks small enough for Vertex AI batch limits
# Skip any file larger than 500 KB or obviously binary assets
MAX_FILE_BYTES = 500 * 1024
BINARY_EXTS = {
    ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico",
    ".pdf", ".svg", ".wasm", ".zip", ".gz", ".tar", ".tgz", ".bz2",
    ".7z", ".exe", ".dll", ".so", ".dylib", ".bin", ".dat", ".mp4",
    ".mp3", ".wav", ".ogg", ".mov"
}
# Safety slice for minified / long‑token chunks
MAX_CHUNK_BYTES = 32 * 1024  # 32 KiB
# Gemini online embedding quotas are limited to ~64 k tokens/min.
# Keep each metadata string small and pace requests.
MAX_METADATA_BYTES = 32 * 1024      # 32 KiB per repo
EMBED_SLEEP = 30                    # seconds to wait between requests (quota-safe)

# ------------------------------------------------------------------
# Helper: embed with retry & exponential back-off to survive quota hits
# ------------------------------------------------------------------
def embed_with_retry(model: TextEmbeddingModel,
                     text: str,
                     base_wait: int = EMBED_SLEEP,
                     max_total_wait: int = 15 * 60) -> list[float] | None:
    """
    Call model.get_embeddings([text]) with exponential back-off until it
    succeeds or max_total_wait seconds have elapsed.

    Returns the embedding list, or None if it ultimately fails.
    """
    total_wait = 0
    wait = base_wait
    attempt = 0
    while True:
        attempt += 1
        try:
            return model.get_embeddings([text])[0].values
        except Exception as exc:
            if "RESOURCE_EXHAUSTED" not in str(exc):
                raise  # Not a quota error → propagate
            if total_wait >= max_total_wait:
                print(f"[ERROR] Giving up on embedding after {attempt} attempts "
                      f"({total_wait}s total wait); skipping.")
                return None
            print(f"[WARN] Quota hit (attempt {attempt}); sleeping {wait}s …")
            time.sleep(wait)
            total_wait += wait
            wait = min(wait * 2, 5 * 60)  # cap individual wait at 5 min
METADATA_MODEL_NAME = f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/gemini-embedding-001"
CODE_MODEL_NAME = f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/text-embedding-005"

MONGO_DB = os.getenv("MONGO_DB_NAME", "repos")

# Initialize clients
storage_client = storage.Client(project=PROJECT_ID)
aiplatform.init(project=PROJECT_ID, location=LOCATION)
mongo_client = MongoClient(MONGO_URI, tls=True, tlsCAFile=certifi.where())
db = mongo_client[MONGO_DB]

# Local embedding models
metadata_embedder = SentenceTransformer('all-mpnet-base-v2')
code_embedder = SentenceTransformer("intfloat/multilingual-e5-large", trust_remote_code=True)
try:
    code_embedder.half()
    code_embedder.quantize(8)
except Exception as e:
    print(f"[WARN] Could not enable half precision or quantization: {e}")

# Debug: list primary DB collections before ingestion
try:
    print(f"[DEBUG] Primary DB name: {MONGO_DB}")
    print(f"[DEBUG] Collections before run: {db.list_collection_names()}")
except Exception as e:
    print(f"[DEBUG] Error listing primary collections: {e}")

# Federated metadata client
# FEDERATED_DB_NAME = os.getenv("FEDERATED_DB_NAME", "repoinstance")
# federated_client = MongoClient(FEDERATED_MONGO_URI, tls=True, tlsCAFile=certifi.where())
# # Debug: list all databases available via federation
# try:
#     print(f"[DEBUG] Federated databases: {federated_client.list_database_names()}")
# except Exception as e:
#     print(f"[DEBUG] Error listing federated databases: {e}")
# federated_db = federated_client[FEDERATED_DB_NAME]
# federated_meta_coll = federated_db["repos_meta"]
#
# # Debug: show federated DB and collections
# print(f"[DEBUG] Federated DB name: {FEDERATED_DB_NAME}")
# try:
#     print(f"[DEBUG] Collections available: {federated_db.list_collection_names()}")
#     count_docs = federated_meta_coll.count_documents({})
#     print(f"[DEBUG] repos_meta document count: {count_docs}")
# except Exception as e:
#     print(f"[DEBUG] Error checking federated collections: {e}")

def upload_jsonl_to_gcs(local_path: str, gcs_path: str):
    bucket = storage_client.bucket(GCS_BUCKET)
    blob = bucket.blob(gcs_path)
    print(f"Uploading {local_path} -> gs://{GCS_BUCKET}/{gcs_path}")
    blob.upload_from_filename(local_path)
    
def download_text_blob(bucket_name: str, blob_name: str) -> str:
    blob = storage_client.bucket(bucket_name).blob(blob_name)
    return blob.download_as_text()

def run_batch_job(input_gcs: str, output_prefix: str, display_name: str, job_type: str):
    print(f"Starting batch prediction: {display_name}")
    if job_type == "metadata":
        job = aiplatform.BatchPredictionJob.create(
            job_display_name=display_name,
            model_name=METADATA_MODEL_NAME,
            instances_format="jsonl",
            predictions_format="jsonl",
            gcs_source=[f"gs://{GCS_BUCKET}/{input_gcs}"],
            gcs_destination_prefix=f"gs://{GCS_BUCKET}/{output_prefix}",
        )
    else:
        job = aiplatform.BatchPredictionJob.create(
        job_display_name=display_name,
        model_name=CODE_MODEL_NAME,
        instances_format="jsonl",
        predictions_format="jsonl",
        gcs_source=[f"gs://{GCS_BUCKET}/{input_gcs}"],
        gcs_destination_prefix=f"gs://{GCS_BUCKET}/{output_prefix}",
        )
    job.wait()
    output_dir = job.output_info.gcs_output_directory  # e.g. gs://bucket/output/repos_code/1234567890
    print(f"Batch prediction {display_name} completed at {output_dir}")
    # Strip the bucket prefix so we can reuse it as a relative path
    return output_dir.replace(f"gs://{GCS_BUCKET}/", "")

def get_repo_id_from_chunk_id(chunk_id: str) -> str:
    """
    Extracts the repo_id (e.g., 'owner/repo') from a code chunk _id.
    Assumes _id format: 'owner/repo/path/to/file::chunk_idx' or 'repo_name/path/to/file::chunk_idx'.
    """
    # First, remove the '::chunk_idx' suffix if present
    base_path = chunk_id.split('::')[0]

    # Split by '/'
    parts = base_path.split('/')

    # If it's 'owner/repo/path' form, take 'owner/repo'
    if len(parts) >= 2:
        return f"{parts[0]}/{parts[1]}"
    # If it's just 'repo_name/path' form, take 'repo_name'
    elif len(parts) == 1:
        return parts[0]
    else: # Should not happen for valid chunk_ids
        return ""

def ingest_predictions_to_mongo_fast(output_prefix: str,
                                     collection_name: str,
                                     key_iter,
                                     batch_size: int = 1000):
    """
    Streaming, batched, unordered upsert of embeddings with ~10× fewer round‑trips.

    Parameters
    ----------
    output_prefix : str
        Folder (relative to bucket) with `predictions_*.jsonl`.
    collection_name : str
        Target Mongo collection.
    key_iter : iterable[str]
        Iterator of identifiers in the same order as the prediction lines.
    batch_size : int
        Number of upserts per bulk_write call.
    """
    coll = db.get_collection(collection_name,
                             write_concern=WriteConcern(w=1, j=False))

    bucket = storage_client.bucket(GCS_BUCKET)
    shards = sorted(
        (
            b
            for b in bucket.list_blobs(prefix=f"{output_prefix}/")
            if b.name.endswith(".jsonl")
            and b.name.count('/') == output_prefix.count('/') + 1
        ),
        key=lambda bl: bl.name,  # sort by filename; avoids TypeError on Blob objects
    )

    line_iter = (ln for bl in shards for ln in bl.download_as_text().splitlines())

    bulk: list[ReplaceOne] = []
    inserted = 0
    keys = list(key_iter)
    total_pred = 0
    for i, line in enumerate(line_iter):
        total_pred += 1
        if i >= len(keys):
            print(f"[WARN] prediction row {i} has no matching key; skipping")
            continue

        # Parse the raw_key as JSON
        key_obj = json.loads(keys[i])
        real_id = key_obj["_id"]
        original_file_path = key_obj["file"]
        original_text = key_obj["text"]
        
        if collection_name == "repos_meta": # Original logic for metadata
            if "|" in real_id:
                # Metadata keys: '00000042|owner/repo' → owner/repo
                real_id = real_id.split("|", 1)[1]
            else:
                # Metadata keys already have the final id
                real_id = real_id
            repo_id = None # Not applicable for repos_meta itself
        else: # Logic for repos_code
            repo_id = get_repo_id_from_chunk_id(real_id).replace("--", "/")

        emb = json.loads(line)["predictions"][0]["embeddings"]["values"]

        doc = {
            "_id": real_id,
            "embedding": emb,
        }
        if repo_id: # Only add repo_id if it's extracted (i.e., for code chunks)
            doc["repo_id"] = repo_id.replace("--", "/")
        
        # Add the original file path and text for repos_code
        if collection_name == "repos_code":
            doc["file"] = original_file_path
            doc["text"] = original_text

        bulk.append(ReplaceOne({"_id": real_id}, doc, upsert=True))
        if len(bulk) >= batch_size:
            coll.bulk_write(bulk, ordered=False)
            inserted += len(bulk)
            bulk.clear()

    if bulk:
        coll.bulk_write(bulk, ordered=False)
        inserted += len(bulk)

    print(f"[FAST] Inserted {inserted} docs into '{collection_name}' "
          f"(predictions seen: {total_pred}, keys: {len(keys)})")

def stringify_repo(repo: dict) -> str:
    """
    Flatten every value in the repo dict into a single space‑separated string.
    Lists and nested dicts are traversed recursively so all primitive values
    (str, int, float, bool) contribute tokens for embedding.
    """
    parts: list[str] = []

    def walk(value):
        if value is None:
            return
        if isinstance(value, str):
            parts.append(value)
        elif isinstance(value, (int, float, bool)):
            parts.append(str(value))
        elif isinstance(value, list):
            for v in value:
                walk(v)
        elif isinstance(value, dict):
            for v in value.values():
                walk(v)

    walk(repo)
    return " ".join(parts)

def chunk_text_by_token_limit(text: str, max_tokens: int = MAX_CODE_TOKENS) -> list[str]:
    """
    Naively chunks text into lists of words, each chunk up to max_tokens words.
    """
    words = text.split()
    chunks = []
    current = []
    for word in words:
        current.append(word)
        if len(current) >= max_tokens:
            chunks.append(" ".join(current))
            current = []
    if current:
        chunks.append(" ".join(current))
    return chunks

def safe_code_chunks(text: str) -> list[str]:
    """
    Returns chunks that are <= MAX_CODE_TOKENS *and* <= MAX_CHUNK_BYTES.
    Handles minified code with few spaces by slicing on bytes.
    """
    primary = chunk_text_by_token_limit(text, max_tokens=MAX_CODE_TOKENS)
    safe: list[str] = []
    for chunk in primary:
        b = chunk.encode("utf-8", errors="ignore")
        if len(b) <= MAX_CHUNK_BYTES:
            safe.append(chunk)
        else:
            # Slice hard every MAX_CHUNK_BYTES bytes
            for i in range(0, len(b), MAX_CHUNK_BYTES):
                part_bytes = b[i : i + MAX_CHUNK_BYTES]
                safe.append(part_bytes.decode("utf-8", errors="ignore"))
    return safe

def main():
    overall_start = time.perf_counter()
    # Reset code collection so we start fresh every run, and clear repos_meta
    db["repos_code"].delete_many({})
    print("[DEBUG] Cleared repos_code collection")
    db["repos_meta"].delete_many({})
    print("[DEBUG] Cleared repos_meta collection")

    # Use local embedding model for metadata
    embedding_model = metadata_embedder

    # ────────────────────────────────────────────────────────────────
    # Metadata embeddings – local embedding
    # ────────────────────────────────────────────────────────────────
    meta_start = time.perf_counter()

    docs_to_insert = []

    def yield_repo_objects(cursor):
        """Yield each repo dict regardless of wrapper shape."""
        for doc in cursor:
            if isinstance(doc, dict) and ("name" in doc or "description" in doc):
                yield doc
                continue
            if isinstance(doc, dict):
                for val in doc.values():
                    if isinstance(val, list):
                        for maybe_repo in val:
                            if isinstance(maybe_repo, dict):
                                yield maybe_repo

    with open("repos.json", "r") as f:
        json_data = json.load(f)
    for obj in yield_repo_objects(json_data):
        text = stringify_repo(obj)
        if not text.strip():
            continue

        token_count = len(text.split())

        if len(text.encode("utf-8")) > MAX_METADATA_BYTES:
            text = text.encode("utf-8")[:MAX_METADATA_BYTES].decode("utf-8", errors="ignore")

        _id = (
            obj.get("full_name")
            or obj.get("name")
            or str(obj.get("_id") or os.urandom(8).hex())
        )

        # Skip if already embedded
        if db["repos_meta"].find_one({"_id": _id}):
            continue

        emb = embedding_model.encode(text, normalize_embeddings=True).tolist()
        print(f"[EMBED] {_id}: {token_count} tokens embedded")

        docs_to_insert.append({"_id": _id, "embedding": emb})

    if docs_to_insert:
        db["repos_meta"].insert_many(docs_to_insert, ordered=False)
        print(f"[INFO] Inserted {len(docs_to_insert)} metadata embeddings (local model).")
    else:
        print("[WARN] No metadata docs to embed.")

    meta_elapsed = time.perf_counter() - meta_start
    print(f"[TIME] Metadata section took {meta_elapsed:.2f}s (local model)")

    code_start = time.perf_counter()
    # 2. Code embeddings
    from concurrent.futures import ThreadPoolExecutor, as_completed

    MAX_PARALLEL_REPOS = 3  # Allow up to X repos in parallel

    def process_repo(repo_dir):
        if not repo_dir.is_dir():
            return 0
        repo_start = time.perf_counter()
        print(f"[PROCESS] Embedding repo: {repo_dir.name}")
        repo_name = repo_dir.name.replace("--", "/")
        file_count = 0
        chunk_count = 0

        def handle_file(file_path):
            nonlocal file_count, chunk_count
            if not file_path.is_file():
                return [], [], [], []
            if file_path.suffix.lower() in BINARY_EXTS:
                return [], [], [], []
            # Remove MAX_FILE_BYTES check and implement new logic
            # New logic: ≤1MB = single chunk, >1MB = chunked
            if file_path.stat().st_size <= 1 * 1024 * 1024:
                # For files ≤ 1MB, read entire content as one chunk
                text = file_path.read_text(encoding="utf-8", errors="ignore")
                boosted_chunk = f"[FILE: {file_path.name}] [PATH: {file_path.relative_to(repo_dir.parent)}]\n{text}"
                chunks = [boosted_chunk]
            else:
                # For files > 1MB, read and chunk
                text = file_path.read_text(encoding="utf-8", errors="ignore")
                raw_chunks = safe_code_chunks(text)
                chunks = [f"[FILE: {file_path.name}] [PATH: {file_path.relative_to(repo_dir.parent)}]\n{chunk}" for chunk in raw_chunks]

            chunk_local_ids = []
            chunk_local_files = []
            chunk_local_texts = []

            for idx, chunk in enumerate(chunks):
                rel_path = file_path.relative_to(repo_dir.parent)
                path_str = str(rel_path).replace("--", "/")
                chunk_id = f"{repo_name.replace('--', '/')}/{path_str}::chunk_{idx}"
                chunk_local_ids.append(chunk_id)
                chunk_local_files.append(path_str)
                chunk_local_texts.append(chunk.replace(str(rel_path), path_str))

            return chunks, chunk_local_ids, chunk_local_files, chunk_local_texts

        all_chunks = []
        chunk_ids = []
        chunk_files = []
        chunk_texts = []

        files = [
            f for f in repo_dir.rglob("*.*")
            if not any(part.startswith('.') for part in f.relative_to(repo_dir).parts)
        ]
        file_count = len(files)

        with ThreadPoolExecutor(max_workers=8) as inner_executor:
            file_results = inner_executor.map(handle_file, files)
            for chunks, ids, files_, texts in file_results:
                all_chunks.extend(chunks)
                chunk_ids.extend(ids)
                chunk_files.extend(files_)
                chunk_texts.extend(texts)
                chunk_count += len(chunks)

        inserted = 0
        if all_chunks:
            embeddings = []
            batch_size = 64
            for i in range(0, len(all_chunks), batch_size):
                try:
                    batch = code_embedder.encode(all_chunks[i:i+batch_size], normalize_embeddings=True)
                    embeddings.extend(batch)
                except Exception as e:
                    print(f"[ERROR] Embedding batch {i//batch_size} failed: {e}")
                    embeddings.extend([None] * len(all_chunks[i:i+batch_size]))
            for i, emb in enumerate(embeddings):
                if emb is None:
                    continue
                chunk_id = chunk_ids[i]
                if db["repos_code"].find_one({"_id": chunk_id}):
                    continue
                doc = {
                    "_id": chunk_id,
                    "repo_id": repo_name,
                    "file": chunk_files[i],
                    "text": chunk_texts[i],
                    "embedding": emb.tolist()
                }
                db["repos_code"].replace_one({"_id": chunk_id}, doc, upsert=True)
                inserted += 1
        print(f"[EMBEDDED] {repo_dir.name}: {file_count} files, {chunk_count} chunks")
        repo_elapsed = time.perf_counter() - repo_start
        print(f"[TIME] {repo_dir.name} took {repo_elapsed:.2f}s")
        return inserted

    repo_dirs = list(Path("./repos").iterdir())

    def process_repo_wrapper(repo_dir):
        try:
            return process_repo(repo_dir)
        except Exception as e:
            print(f"[ERROR] Failed processing {repo_dir.name}: {e}")
            return 0

    code_embeddings_inserted = 0
    with ThreadPoolExecutor(max_workers=MAX_PARALLEL_REPOS) as outer_executor:
        futures = {outer_executor.submit(process_repo_wrapper, repo): repo for repo in repo_dirs}
        for future in as_completed(futures):
            result = future.result()
            code_embeddings_inserted += result

    print(f"[INFO] Inserted code embeddings directly via local model.")

    code_elapsed = time.perf_counter() - code_start
    print(f"[TIME] Code section took {code_elapsed:.2f} s")

    # Final debug: list primary DB collections and counts
    try:
        print(f"[DEBUG] Collections after run: {db.list_collection_names()}")
        for coll in db.list_collection_names():
            cnt = db[coll].count_documents({})
            print(f"[DEBUG] {coll}: {cnt} documents")
    except Exception as e:
        print(f"[DEBUG] Error during final debug listing: {e}")

    overall_elapsed = time.perf_counter() - overall_start
    print(f"[TIME] Total runtime {overall_elapsed:.2f} s")

if __name__ == "__main__":
    main()