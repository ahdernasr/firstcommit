#!/usr/bin/env python3
import os, json, certifi
from google.cloud import storage
from dotenv import load_dotenv
from pymongo import MongoClient, ReplaceOne, WriteConcern

# ───------------- customize these two values ------------────────
OUTPUT_DIR = "output/repos_code/prediction-model-2025-06-14T15:45:32.043631Z"
BUCKET     = "ai-in-action-repo-bucket"
# ────────────────────────────────────────────────────────────────

load_dotenv("./.env")

# MongoDB connection (same URI you use in embed.py)
MONGO_URI = os.getenv("MONGODB_URI")
client = MongoClient(MONGO_URI, tls=True, tlsCAFile=certifi.where())
db     = client[ os.getenv("MONGO_DB_NAME", "repos") ]
coll   = db.get_collection("repos_code",
                           write_concern=WriteConcern(w=1, j=False))

storage_client = storage.Client()
bucket = storage_client.bucket(BUCKET)

# Download the key manifest (ids must match Vertex output line-order)
key_blob = bucket.blob("input/repos_code.keys")
keys = key_blob.download_as_text().splitlines()
key_iter = iter(keys)

# Stream through every predictions_-shard-.jsonl in order
shards = sorted(
    (
        b for b in bucket.list_blobs(prefix=f"{OUTPUT_DIR}/")
        if b.name.endswith(".jsonl")
        and b.name.count("/") == OUTPUT_DIR.count("/") + 1
    ),
    key=lambda bl: bl.name,            # ensure deterministic order
)

# ─── Helper to cope with different Vertex output shapes ────────────────
def extract_embedding(json_line: str) -> list[float]:
    """
    Return the embedding vector regardless of whether Vertex wraps it in
    'predictions', 'prediction', or gives the embedding dict directly.
    """
    obj = json.loads(json_line)
    if "predictions" in obj:          # common case
        return obj["predictions"][0]["embeddings"]["values"]
    if "prediction" in obj:           # singular wrapper
        return obj["prediction"]["embeddings"]["values"]
    if "embeddings" in obj:           # raw dict
        return obj["embeddings"]["values"]
    raise KeyError(f"Unexpected output shape: {list(obj.keys())}")
# ───────────────────────────────────────────────────────────────────────

batch, inserted = [], 0
for blob in shards:
    for line in blob.download_as_text().splitlines():
        emb = extract_embedding(line)
        doc_id = next(key_iter)        # one-to-one with prediction line
        batch.append(
            ReplaceOne({"_id": doc_id},
                       {"_id": doc_id, "embedding": emb},
                       upsert=True)
        )
        if len(batch) >= 1_000:
            coll.bulk_write(batch, ordered=False)
            inserted += len(batch)
            batch.clear()

# flush leftovers
if batch:
    coll.bulk_write(batch, ordered=False)
    inserted += len(batch)

print(f"Inserted {inserted} embeddings into repos_code")