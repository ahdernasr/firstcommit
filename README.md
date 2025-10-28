# FirstCommit

FirstCommit helps developers contribute to open source faster by combining semantic search, AI-guided issue breakdowns, and intelligent context retrieval. It is designed to reduce the time between discovering a repository and making a meaningful contribution.

This project was built for the Google Cloud x MongoDB AI in Action Hackathon.

## What It Does

FirstCommit allows developers to

* Search and discover active GitHub repositories using semantic search powered by vector embeddings
* Explore open issues with linked code snippets, relevant files, and commit history
* Get step-by-step guidance on how to resolve issues using a retrieval-augmented generation (RAG) pipeline
* Understand the structure of large codebases without needing to read every file

## Tech Stack

**Frontend**  
Next.js, Tailwind CSS, deployed on Vercel

**Backend**  
Go (Fiber), REST APIs

**Database**  
MongoDB Atlas with vector search, Atlas Data Federation

**AI and Infrastructure**  
Vertex AI (Gemini API), Google Cloud Storage, Batch Prediction, BigQuery, Dataflow

## Key Features

### Semantic Repository Search

Developers can find repositories to contribute to based on natural language input. The search leverages vector embeddings of repository metadata and README files.

### Issue Contextualization

When a developer selects an issue, FirstIssue retrieves and displays the most relevant parts of the codebase. This is done using semantic search over chunked code embeddings.

### AI-Powered Guidance

Each issue page includes a generated guide that walks through the expected resolution steps. This includes an outline of what the issue is about, which files or functions are involved, and how to get started.

### Pipeline

1. GitHub data is pulled from GH Archive and queried via BigQuery
2. Repositories are cloned and parsed into code chunks
3. Embeddings are generated using Vertex AI and stored in MongoDB
4. When a query is made, relevant chunks are retrieved using Atlas vector search
5. A prompt is constructed with retrieved content and passed to Gemini for explanation

## Setup Instructions

To run locally:

```bash
git clone https://github.com/ahmednasr/firstissue.git
cd firstissue

# Backend
cd server
go run main.go

# Frontend
cd ../web
npm install
npm run dev
