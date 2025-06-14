#!/usr/bin/env python3
import os
import sys
import json
from pathlib import Path
from dotenv import load_dotenv
from google.cloud import storage
import google.cloud.aiplatform as aiplatform
from pymongo import MongoClient
from pymongo import ReplaceOne, WriteConcern
import certifi
import time

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
MODEL_NAME = f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/text-embedding-005"

MONGO_DB = os.getenv("MONGO_DB_NAME", "repos")

# Initialize clients
storage_client = storage.Client(project=PROJECT_ID)
aiplatform.init(project=PROJECT_ID, location=LOCATION)
mongo_client = MongoClient(MONGO_URI, tls=True, tlsCAFile=certifi.where())
db = mongo_client[MONGO_DB]

# Debug: list primary DB collections before ingestion
try:
    print(f"[DEBUG] Primary DB name: {MONGO_DB}")
    print(f"[DEBUG] Collections before run: {db.list_collection_names()}")
except Exception as e:
    print(f"[DEBUG] Error listing primary collections: {e}")

# Federated metadata client
FEDERATED_DB_NAME = os.getenv("FEDERATED_DB_NAME", "repoinstance")
federated_client = MongoClient(FEDERATED_MONGO_URI, tls=True, tlsCAFile=certifi.where())
# Debug: list all databases available via federation
try:
    print(f"[DEBUG] Federated databases: {federated_client.list_database_names()}")
except Exception as e:
    print(f"[DEBUG] Error listing federated databases: {e}")
federated_db = federated_client[FEDERATED_DB_NAME]
federated_meta_coll = federated_db["repos_meta"]

# Debug: show federated DB and collections
print(f"[DEBUG] Federated DB name: {FEDERATED_DB_NAME}")
try:
    print(f"[DEBUG] Collections available: {federated_db.list_collection_names()}")
    count_docs = federated_meta_coll.count_documents({})
    print(f"[DEBUG] repos_meta document count: {count_docs}")
except Exception as e:
    print(f"[DEBUG] Error checking federated collections: {e}")

def upload_jsonl_to_gcs(local_path: str, gcs_path: str):
    bucket = storage_client.bucket(GCS_BUCKET)
    blob = bucket.blob(gcs_path)
    print(f"Uploading {local_path} -> gs://{GCS_BUCKET}/{gcs_path}")
    blob.upload_from_filename(local_path)
    
def download_text_blob(bucket_name: str, blob_name: str) -> str:
    blob = storage_client.bucket(bucket_name).blob(blob_name)
    return blob.download_as_text()

def run_batch_job(input_gcs: str, output_prefix: str, display_name: str):
    print(f"Starting batch prediction: {display_name}")
    job = aiplatform.BatchPredictionJob.create(
        job_display_name=display_name,
        model_name=MODEL_NAME,
        instances_format="jsonl",
        predictions_format="jsonl",
        gcs_source=[f"gs://{GCS_BUCKET}/{input_gcs}"],
        gcs_destination_prefix=f"gs://{GCS_BUCKET}/{output_prefix}"
    )
    job.wait()
    output_dir = job.output_info.gcs_output_directory  # e.g. gs://bucket/output/repos_code/1234567890
    print(f"Batch prediction {display_name} completed at {output_dir}")
    # Strip the bucket prefix so we can reuse it as a relative path
    return output_dir.replace(f"gs://{GCS_BUCKET}/", "")

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
        b for b in bucket.list_blobs(prefix=f"{output_prefix}/")
        if b.name.endswith(".jsonl") and b.name.count('/') == output_prefix.count('/') + 1
    )

    line_iter = (ln for bl in shards for ln in bl.download_as_text().splitlines())

    bulk: list[ReplaceOne] = []
    inserted = 0
    for key, line in zip(key_iter, line_iter):
        emb = json.loads(line)["predictions"][0]["embeddings"]["values"]
        bulk.append(
            ReplaceOne({"_id": key},
                       {"_id": key, "embedding": emb},
                       upsert=True)
        )
        if len(bulk) >= batch_size:
            coll.bulk_write(bulk, ordered=False)
            inserted += len(bulk)
            bulk.clear()

    if bulk:
        coll.bulk_write(bulk, ordered=False)
        inserted += len(bulk)

    print(f"[FAST] Inserted {inserted} docs into '{collection_name}'")

def chunk_text_by_token_limit(text: str, max_tokens: int = 15000) -> list[str]:
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

def main():
    overall_start = time.perf_counter()
    # Reset collections so we start fresh every run
    db["repos_code"].delete_many({})
    db["repos_meta"].delete_many({})
    print("[DEBUG] Cleared repos_code and repos_meta collections")

    meta_start = time.perf_counter()
    # 1. Metadata embeddings from federated collection
    metadata_jsonl = "/tmp/repos_meta.jsonl"
    meta_key_manifest = "/tmp/repos_meta.keys"

    with open(metadata_jsonl, "w", encoding="utf-8") as fout_jsonl, \
         open(meta_key_manifest, "w", encoding="utf-8") as fout_keys:

        for obj in federated_meta_coll.find():
            text = " ".join(filter(None, [obj.get("name"), obj.get("description") or ""]))
            fout_jsonl.write(json.dumps({"content": text}) + "\n")
            key = (
                str(obj.get("_id"))           # Atlas-generated _id if present
                or obj.get("full_name")       # "owner/repo"
                or obj.get("name")            # simple repo name
                or os.urandom(8).hex()        # guaranteed unique fallback
            )
            fout_keys.write(key + "\n")

    # Only proceed if there is data
    line_count = sum(1 for _ in open(metadata_jsonl, "r", encoding="utf-8"))
    if line_count == 0:
        print("No metadata items to embed; skipping metadata batch job.")
    else:
        meta_output_dir = run_batch_job("input/repos_meta.jsonl",
                                        "output/repos_meta",
                                        "embed-metadata")
        meta_keys = download_text_blob(GCS_BUCKET, "input/repos_meta.keys").splitlines()
        ingest_predictions_to_mongo_fast(meta_output_dir,
                                         "repos_meta",
                                         iter(meta_keys))
        meta_elapsed = time.perf_counter() - meta_start
        print(f"[TIME] Metadata section took {meta_elapsed:.2f} s")

    code_start = time.perf_counter()
    # 2. Code embeddings
    code_jsonl = "/tmp/repos_code.jsonl"
    key_manifest = "/tmp/repos_code.keys"

    with open(code_jsonl, "w", encoding="utf-8") as fout_jsonl, \
        open(key_manifest, "w", encoding="utf-8") as fout_keys:

        for repo_dir in (BASE_DIR / "repos").iterdir():
            if not repo_dir.is_dir():
                continue
            for file_path in repo_dir.rglob("*.*"):
                if not file_path.is_file():
                    continue

                text = file_path.read_text(encoding="utf-8", errors="ignore")
                chunks = chunk_text_by_token_limit(text, max_tokens=15000)

                for idx, chunk in enumerate(chunks):
                    key = f"{repo_dir.name}/{file_path.relative_to(repo_dir.parent)}::chunk_{idx}"

                    # Write the chunk for Vertex AI (content-only)
                    fout_jsonl.write(json.dumps({"content": chunk}) + "\n")

                    # Record the identifier on the matching line in the manifest
                    fout_keys.write(key + "\n")
    line_count = sum(1 for _ in open(code_jsonl, "r", encoding="utf-8"))
    if line_count == 0:
        print("No code chunks to embed; skipping code batch job.")
    else:
        upload_jsonl_to_gcs(code_jsonl, "input/repos_code.jsonl")
        upload_jsonl_to_gcs(key_manifest, "input/repos_code.keys")
        code_output_dir = run_batch_job("input/repos_code.jsonl",
                                        "output/repos_code",
                                        "embed-code")
        code_keys = download_text_blob(GCS_BUCKET, "input/repos_code.keys").splitlines()
        ingest_predictions_to_mongo_fast(code_output_dir,
                                         "repos_code",
                                         iter(code_keys))
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