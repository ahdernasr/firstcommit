#!/usr/bin/env python3
"""
search.py — simple CLI to run vector search against `repos_code` or `repos_meta`
collections in MongoDB Atlas.

Examples
--------
# Search repository metadata
python search.py -c repos_meta -q "graphql subscription filter" -k 5

# Search code chunks
python search.py --collection repos_code --query "open csv file in go" --k 10
"""
import argparse
import json
import os
import sys
from pathlib import Path

import certifi
from dotenv import load_dotenv
from google.cloud import aiplatform
from pymongo import MongoClient

# ──────────────── environment & clients ─────────────────
BASE_DIR = Path(__file__).resolve().parent
load_dotenv("./.env")

PROJECT_ID = os.getenv("GCP_PROJECT_ID")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")
MODEL_NAME = (
    f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/"
    "models/text-embedding-005"
)

MONGO_URI = os.getenv("MONGODB_URI")
MONGO_DB = os.getenv("MONGO_DB_NAME", "repos")

if not PROJECT_ID or not MONGO_URI:
    sys.stderr.write(
        "Error: GCP_PROJECT_ID and MONGODB_URI must be set in your .env file\n"
    )
    sys.exit(1)

aiplatform.init(project=PROJECT_ID, location=LOCATION)
mongo = MongoClient(MONGO_URI, tls=True, tlsCAFile=certifi.where())
db = mongo[MONGO_DB]


# ──────────────── helper functions ──────────────────────
def embed_text(text: str) -> list[float]:
    """Embed a single string using Vertex AI text‑embedding‑005."""
    client = aiplatform.gapic.PredictionServiceClient(
        client_options={"api_endpoint": f"{LOCATION}-aiplatform.googleapis.com"}
    )
    resp = client.predict(
        endpoint=MODEL_NAME,
        instances=[{"content": text}],
        parameters={},
    )
    # Vertex AI returns a protobuf RepeatedScalarField; cast to plain list
    values = resp.predictions[0]["embeddings"]["values"]  # type: ignore
    return [float(x) for x in values]


def vector_search(
    collection: str,
    query_vec: list[float],
    k: int = 10,
) -> list[dict]:
    """Run a $vectorSearch aggregation and return the top‑k documents."""
    pipeline = [
        {
            "$vectorSearch": {
                "index": "vector_index",
                "path": "embedding",
                "queryVector": query_vec,
                "numCandidates": k * 10,
                "limit": k,
                "similarity": "cosine",
            }
        },
        {
            "$project": {
                "_id": 1,
                "name": 1,
                "description": 1,
                "file": 1,  # only present in repos_code
                "score": {"$meta": "vectorSearchScore"},
            }
        },
    ]
    return list(db[collection].aggregate(pipeline))


# ──────────────── CLI parsing & entry point ─────────────
def parse_args() -> argparse.Namespace:
    ap = argparse.ArgumentParser(description="Vector search CLI.")
    ap.add_argument(
        "-c",
        "--collection",
        choices=["repos_code", "repos_meta"],
        required=True,
        help="Target collection to search.",
    )
    ap.add_argument("-q", "--query", required=True, help="Natural‑language query.")
    ap.add_argument(
        "-k",
        type=int,
        default=10,
        help="Number of results to return (default 10)."
    )
    return ap.parse_args()


def main() -> None:
    args = parse_args()

    print(f"Embedding query: {args.query!r}")
    q_vec = embed_text(args.query)

    print(f"Running vector search on '{args.collection}' …")
    hits = vector_search(args.collection, q_vec, args.k)

    if not hits:
        print("No results found.")
        return

    for rank, doc in enumerate(hits, 1):
        score = doc.pop("score", 0)
        print(f"\n#{rank}  (score={score:.4f})")
        print(json.dumps(doc, indent=2, default=str)[:800])  # truncate long fields


if __name__ == "__main__":
    main()
