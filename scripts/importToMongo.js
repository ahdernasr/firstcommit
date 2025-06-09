import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { MongoClient } from "mongodb";
import fetch from "node-fetch"  ;
import { GoogleAuth } from "google-auth-library";
import dotenv from "dotenv";
dotenv.config();

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// === CONFIG ===
const MONGO_URI = process.env.MONGODB_URI;
console.log("MONGODB_URI is:", MONGO_URI);
const client = new MongoClient(MONGO_URI);
const db = client.db("firstcommit");
const collection = db.collection("repositories");
const keyPath = "./vertex-key.json";
// Using Vertex AI public embedding model gemini-embedding-001; ensure GenAI API is enabled.
const EMBEDDING_ENDPOINT =
  "https://us-central1-aiplatform.googleapis.com/v1/projects/ai-in-action-461204/locations/us-central1/publishers/google/models/gemini-embedding-001:predict";
  
// === AUTH ===
const auth = new GoogleAuth({
  keyFile: keyPath,
  scopes: "https://www.googleapis.com/auth/cloud-platform",
});

const getEmbedding = async (text) => {
  try {
    const client = await auth.getClient();
    const accessToken = await client.getAccessToken();

    const res = await fetch(EMBEDDING_ENDPOINT, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        instances: [{ content: text }],
      }),
    });

    if (!res.ok) {
      const errorText = await res.text();
      console.error(`❌ Embedding API error [${res.status}]: ${errorText}`);
      throw new Error(`Embedding API returned ${res.status}`);
    }

    let data;
    try {
      data = await res.json();
    } catch (parseErr) {
      console.error(`❌ Failed to parse JSON from Embedding API: ${parseErr.message}`);
      throw new Error(`Failed to parse JSON from Embedding API: ${parseErr.message}`);
    }

    let vector;
    if (Array.isArray(data.predictions) && data.predictions[0]?.values) {
      vector = data.predictions[0].values;
    } else if (Array.isArray(data.predictions) && data.predictions[0]?.embeddings?.values) {
      vector = data.predictions[0].embeddings.values;
    } else {
      console.error("❌ Vertex response:", JSON.stringify(data, null, 2));
      throw new Error("Invalid embedding response format");
    }
    return vector;
  } catch (err) {
    throw new Error("Embedding fetch failed: " + err.message);
  }
};

// === MAIN ===
const main = async () => {
  const raw = fs.readFileSync(path.resolve(__dirname, "../../FirstCommit/dataset/repos.json"), "utf8");
  const repos = JSON.parse(raw);

  console.log(`🔍 Loaded ${repos.length} repos from file`);

  await collection.deleteMany({});
  console.log("🗑️ Cleared existing documents in the collection.");

  let inserted = 0;
  for (const repo of repos) {
    try {
      const desc = [
        repo.description,
        repo.readme,
        (repo.topics || []).join(" "),
        repo.language,
        repo.full_name
      ].filter(Boolean).join(" — ");
      console.log("📝 Embedding description:", desc);
      const embedding = await getEmbedding(desc);

      const doc = {
        name: repo.name,
        full_name: repo.full_name,
        owner: repo.owner?.login,
        html_url: repo.html_url,
        description: repo.description,
        language: repo.language,
        stargazers_count: repo.stargazers_count,
        watchers_count: repo.watchers_count,
        forks_count: repo.forks_count,
        open_issues_count: repo.open_issues_count,
        license: repo.license?.spdx_id || null,
        homepage: repo.homepage,
        default_branch: repo.default_branch,
        created_at: repo.created_at,
        pushed_at: repo.pushed_at,
        size: repo.size,
        visibility: repo.visibility,
        archived: repo.archived,
        allow_forking: repo.allow_forking,
        is_template: repo.is_template,
        has_wiki: repo.has_wiki,
        has_pages: repo.has_pages,
        has_discussions: repo.has_discussions,
        topics: repo.topics || [],
        languages: repo.languages || {},
        readme: repo.readme || "",
        score: repo.score || 0,
        relevance_reason: repo.relevance_reason || "",
        embedding
      };

      await collection.replaceOne({ full_name: repo.full_name }, doc, { upsert: true });
      inserted++;
      console.log(`✅ Inserted ${repo.full_name}`);
    } catch (err) {
      console.error(`❌ Failed to process ${repo.full_name || repo.name}: ${err.message}`);
    }
  }

  console.log(`🎯 Finished. Successfully inserted ${inserted} repositories.`);

  await client.close();
};

main();