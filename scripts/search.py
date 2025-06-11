#!/usr/bin/env python3

import os
import sys
import json
from dotenv import load_dotenv
import requests
from google.oauth2 import service_account
from google.auth.transport.requests import Request
from pymongo import MongoClient
import certifi

# Load environment variables
load_dotenv(dotenv_path=os.path.join(os.path.dirname(__file__), '.env'))

# Config
PROJECT_ID = os.getenv("GCP_PROJECT_ID")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")
MODEL = os.getenv("EMBEDDING_MODEL")
KEYFILE = os.getenv("GCP_KEYFILE_PATH")
MONGODB_URI = os.getenv("MONGODB_URI")
DB_NAME = os.getenv("DB_NAME", "vector_db")
COLLECTION_NAME = os.getenv("COLLECTION_NAME", "repos")

# Authenticate and get access token
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
collection = db[COLLECTION_NAME]

def get_embedding(text: str):
    """Generate embedding vector for the given text via Vertex AI REST."""
    endpoint = (
        f"https://{LOCATION}-aiplatform.googleapis.com/v1/"
        f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/"
        f"models/{MODEL}:predict"
    )
    payload = {
        "instances": [
            {
                "task_type": "RETRIEVAL_DOCUMENT",
                "title": "query",
                "content": text
            }
        ]
    }
    headers = {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json; charset=utf-8"
    }
    response = requests.post(endpoint, headers=headers, json=payload)
    if not response.ok:
        print("Embedding request failed:", response.text)
        response.raise_for_status()
    result = response.json()
    return result["predictions"][0]["embeddings"]["values"]

def vector_search(query: str, k: int = 10):
    """Run a vector similarity search on MongoDB."""
    vec = get_embedding(query)
    pipeline = [
        {
            "$vectorSearch": {
                "index": "default",
                "queryVector": vec,
                "path": "embedding",
                "limit": k,
                "numCandidates": 500
            }
        },
        {"$project": {"full_name": 1, "description": 1, "score": {"$meta": "vectorSearchScore"}}},
        {"$limit": k}
    ]
    results = list(collection.aggregate(pipeline))
    if not results:
        print("No matching results found.")
    else:
        print(f"Found {len(results)} results.\n")
    for idx, doc in enumerate(results, 1):
        print(f"{idx}. {doc.get('full_name', doc.get('name', 'N/A'))} (score: {doc.get('score', 0):.4f})\n   {doc.get('description','')}\n")

def debug_check_embeddings():
    """Print how many documents have embeddings and show one example."""
    count = collection.count_documents({"embedding": {"$exists": True}})
    print(f"Documents with embedding: {count}")
    if count > 0:
        doc = collection.find_one({"embedding": {"$exists": True}})
        print("Example document:")
        print(f"Name: {doc.get('full_name', doc.get('name'))}")
        print(f"First 10 dims: {doc['embedding'][:10]}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python search.py \"search text\" [k]")
        sys.exit(1)
    query = sys.argv[1]
    k = int(sys.argv[2]) if len(sys.argv) > 2 else 10
    print("Running debug check...\n")
    debug_check_embeddings()
    print("\nRunning vector search...\n")
    vector_search(query, k)