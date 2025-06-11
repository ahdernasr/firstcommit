#!/usr/bin/env python3

import os
import json
from dotenv import load_dotenv
import requests
from google.oauth2 import service_account
from google.auth.transport.requests import Request
from pymongo import MongoClient

# Load environment variables from .env
load_dotenv(dotenv_path=os.path.join(os.path.dirname(__file__), '.env'))

# Configuration from environment
PROJECT_ID = os.getenv("GCP_PROJECT_ID")
LOCATION = os.getenv("GCP_LOCATION", "us-central1")
MODEL = os.getenv("EMBEDDING_MODEL")
KEYFILE = os.getenv("GCP_KEYFILE_PATH")
MONGODB_URI = os.getenv("MONGODB_URI")
DB_NAME = os.getenv("DB_NAME", "vector_db")
COLLECTION_NAME = os.getenv("COLLECTION_NAME", "repos")

# Authenticate with service account
credentials = service_account.Credentials.from_service_account_file(
    KEYFILE,
    scopes=["https://www.googleapis.com/auth/cloud-platform"]
)
auth_request = Request()
credentials.refresh(auth_request)
access_token = credentials.token

# Initialize MongoDB client
mongo_client = MongoClient(MONGODB_URI)
db = mongo_client[DB_NAME]
collection = db[COLLECTION_NAME]

def embed_and_insert(repo: dict):
    """
    Generate embedding for repo and insert into MongoDB.
    """
    # Build text to embed
    text_to_embed = " ".join([
        repo.get("name", ""),
        repo.get("description", ""),
        *repo.get("topics", [])
    ])

    # Prepare Vertex AI REST endpoint
    endpoint = (
        f"https://{LOCATION}-aiplatform.googleapis.com/v1/"
        f"projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/"
        f"models/{MODEL}:predict"
    )

    # Prepare request payload
    payload = {
        "instances": [
            {
                "task_type": "RETRIEVAL_DOCUMENT",
                "title": repo.get("name", ""),
                "content": text_to_embed
            }
        ]
    }

    # Call Vertex AI embedding endpoint
    headers = {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json; charset=utf-8"
    }
    response = requests.post(endpoint, headers=headers, json=payload)
    response.raise_for_status()
    result = response.json()

    # Extract embedding vector
    embeddings = result["predictions"][0]["embeddings"]["values"]

    # Add embedding to repo object and insert
    repo["embedding"] = embeddings
    collection.insert_one(repo)
    print(f"Inserted {repo.get('full_name', repo.get('name'))} with embedding.")

if __name__ == "__main__":
    # Load repository list from JSON file
    data_path = os.path.normpath(os.path.join(
        os.path.dirname(__file__),
        "..", "..",
        "FirstCommit", "dataset", "repos.json"
    ))
    with open(data_path, "r", encoding="utf-8") as f:
        repos = json.load(f)

    # Embed and insert each repository
    for repo in repos:
        try:
            embed_and_insert(repo)
        except Exception as e:
            print(f"Failed to embed {repo.get('full_name', repo.get('name'))}: {e}")