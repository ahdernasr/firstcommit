#!/bin/bash

# Exit on error
set -e

# Check for required tools
command -v gcloud >/dev/null 2>&1 || { echo "gcloud is required but not installed. Aborting." >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "docker is required but not installed. Aborting." >&2; exit 1; }

# Load environment variables from each directory
if [ -f "client/.env" ]; then
    source client/.env
fi
if [ -f "server/.env" ]; then
    source server/.env
fi
if [ -f "scripts/.env" ]; then
    source scripts/.env
fi

# Verify GCP project
if [ -z "$GCP_PROJECT_ID" ]; then
    echo "Error: GCP_PROJECT_ID is not set in any .env file"
    exit 1
fi

# Set default values if not provided in .env
REGION=${GCP_REGION:-"us-central1"}
REPO_NAME=${GCP_REPO_NAME:-"ai-in-action"}
VERSION=${VERSION:-"latest"}

# Set GCP project
gcloud config set project $GCP_PROJECT_ID

# Enable required APIs
echo "Enabling required APIs..."
gcloud services enable \
    run.googleapis.com \
    artifactregistry.googleapis.com \
    storage.googleapis.com

# Create Artifact Registry repository if it doesn't exist
if ! gcloud artifacts repositories describe $REPO_NAME --location=$REGION >/dev/null 2>&1; then
    echo "Creating Artifact Registry repository..."
    gcloud artifacts repositories create $REPO_NAME \
        --repository-format=docker \
        --location=$REGION \
        --description="Docker repository for AI in Action"
fi

# Configure Docker to use Google Cloud credentials
echo "Configuring Docker authentication..."
gcloud auth configure-docker $REGION-docker.pkg.dev

# Build and push frontend
echo "Building and pushing frontend..."
cd client
docker build --platform linux/amd64 \
    --build-arg NEXT_PUBLIC_API_URL=$BACKEND_URL \
    -t $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/frontend:$VERSION .
docker push $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/frontend:$VERSION
cd ..

# Build and push backend
echo "Building and pushing backend..."
cd server
docker build --platform linux/amd64 -t $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/backend:$VERSION .
docker push $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/backend:$VERSION
cd ..

# Prepare environment variables for deployment
echo "Preparing environment variables..."

# Frontend env vars
FRONTEND_ENV_VARS=""
while IFS='=' read -r key value; do
    # Skip comments, empty lines, and PORT
    [[ $key =~ ^#.*$ ]] && continue
    [[ -z $key ]] && continue
    [[ $key == "PORT" ]] && continue
    # Remove any quotes from the value
    value=$(echo "$value" | tr -d '"'"'")
    FRONTEND_ENV_VARS="$FRONTEND_ENV_VARS,$key=$value"
done < client/.env
FRONTEND_ENV_VARS=${FRONTEND_ENV_VARS#,}  # Remove leading comma

# Backend env vars
BACKEND_ENV_VARS=""
while IFS='=' read -r key value; do
    # Skip comments, empty lines, and PORT
    [[ $key =~ ^#.*$ ]] && continue
    [[ -z $key ]] && continue
    [[ $key == "PORT" ]] && continue
    # Remove any quotes from the value
    value=$(echo "$value" | tr -d '"'"'")
    BACKEND_ENV_VARS="$BACKEND_ENV_VARS,$key=$value"
done < server/.env
BACKEND_ENV_VARS=${BACKEND_ENV_VARS#,}  # Remove leading comma

# Deploy backend to Cloud Run
echo "Deploying backend to Cloud Run..."
gcloud run deploy backend \
    --image $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/backend:$VERSION \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --port 8080 \
    --set-env-vars="$BACKEND_ENV_VARS"

# Get the backend URL
BACKEND_URL=$(gcloud run services describe backend --platform managed --region $REGION --format 'value(status.url)')
echo "Backend URL: $BACKEND_URL"

# Deploy frontend to Cloud Run with the backend URL
echo "Deploying frontend to Cloud Run..."
gcloud run deploy frontend \
    --image $REGION-docker.pkg.dev/$GCP_PROJECT_ID/$REPO_NAME/frontend:$VERSION \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --port 3000 \
    --set-env-vars="NEXT_PUBLIC_API_URL=$BACKEND_URL,$FRONTEND_ENV_VARS"

echo "Deployment completed successfully!" 