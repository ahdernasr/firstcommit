"use client";

import type React from "react";

import { useState, useEffect } from "react";
import {
  Search,
  Star,
  GitFork,
  Eye,
  Filter,
  ArrowDownUp,
  AlertCircle,
} from "lucide-react"; // Added AlertCircle icon
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import Link from "next/link";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { getApiEndpoint } from '@/lib/config'

interface Repository {
  _id: string; // MongoDB _id
  name: string;
  full_name: string;
  owner: string;
  html_url: string; // Now consistently available
  description?: string; // Make optional as it might be null
  language?: string; // Renamed from languages for simplicity on card display, will pick first from array
  stargazers_count: number; // Updated to match backend field name
  watchers_count: number;
  forks_count: number;
  open_issues_count: number;
  license?: string;
  homepage?: string;
  image_url: string; // Directly from backend's Repo model
  default_branch?: string;
  created_at: string;
  pushed_at: string;
  size: number;
  visibility: string;
  archived: boolean;
  allow_forking: boolean;
  is_template: boolean;
  topics?: string[]; // Make optional
  languages: string[]; // Keep original languages array from backend
  readme?: string; // Add Readme field back to the interface
  score: number;
  relevance_reason?: string;
}

export default function HomePage() {
  const [searchQuery, setSearchQuery] = useState("");
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [loading, setLoading] = useState(false);
  const [sortBy, setSortBy] = useState("stars"); // Corresponds to selectedSort
  const [language, setLanguage] = useState("all"); // Corresponds to selectedLanguage
  const [selectedTopic, setSelectedTopic] = useState("all"); // New state for topic filter

  const [showExamples, setShowExamples] = useState(false); // Changed to false to prevent showing example repos initially

  const searchRepositories = async () => {
    setShowExamples(false);
    setLoading(true);
    try {
      const fullUrl = getApiEndpoint(`/api/v1/search?q=${encodeURIComponent(searchQuery)}`)
      console.log('Searching with URL:', fullUrl)
      
      const response = await fetch(fullUrl, {
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('Search API error:', {
          status: response.status,
          statusText: response.statusText,
          error: errorText
        })
        throw new Error(`HTTP error! status: ${response.status}, message: ${errorText}`)
      }

      const data = await response.json()
      console.log('Search API response:', data)
      setRepositories(data.repositories || [])
      console.log('Set repositories:', data.repositories || [])
    } catch (error) {
      console.error("Error searching repositories:", error)
      setRepositories([])
    } finally {
      setLoading(false)
    }
  };

  const fetchInitialRepositories = async () => {
    setLoading(true);
    try {
      const fullUrl = getApiEndpoint('/api/v1/search?q=stars:>100')
      console.log('Fetching initial repos from:', fullUrl)
      
      const response = await fetch(fullUrl, {
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('Initial load API error:', {
          status: response.status,
          statusText: response.statusText,
          error: errorText
        })
        throw new Error(`HTTP error! status: ${response.status}, message: ${errorText}`)
      }

      const data = await response.json()
      setRepositories(data.repositories || [])
    } catch (error) {
      console.error("Error fetching initial repositories:", error)
      setRepositories([])
    } finally {
      setLoading(false)
    }
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    searchRepositories();
  };

  useEffect(() => {
    // Only trigger search if a query is active
    if (searchQuery.trim()) {
      const debounceTimer = setTimeout(() => {
        searchRepositories();
      }, 500);
      return () => clearTimeout(debounceTimer);
    } else {
      // Fetch initial repositories if no search query is active
      fetchInitialRepositories();
    }
  }, [sortBy, language, selectedTopic, searchQuery]);

  return (
    <div className="min-h-screen bg-[#16191d]">
      <div className="container mx-auto px-6 py-8">
        <div className="text-center mb-12">
          <h1 className="text-5xl text-[#f3c9a4] font-bold mb-6 leading-relaxed py-2 font-oswald">
            GitHub Repository Explorer
          </h1>
          <p className="text-[#f3f3f3]/80 text-xl max-w-2xl mx-auto leading-relaxed">
            Discover and explore open source repositories with AI-powered issue
            guidance
          </p>
        </div>

        {/* Search and Filters */}
        <div className="max-w-4xl mx-auto mb-12">
          <form onSubmit={handleSearch} className="space-y-6">
            <div className="relative">
              <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 text-[#515b65] h-5 w-5" />
              <Input
                placeholder="Search repositories..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-12 pr-32 h-14 text-lg bg-[#292f36] border-[#515b65] rounded-lg focus:ring-0 focus:border-[#515b65] transition-all duration-200 text-[#f3f3f3]"
              />
              <Button
                type="submit"
                disabled={loading}
                className="absolute right-2 top-1/2 transform -translate-y-1/2 h-10 px-6 bg-[#f3c9a4] text-black rounded-lg font-medium shadow-md hover:bg-[#d4a882] transition-all duration-200"
              >
                {loading ? "Searching..." : "Search"}
              </Button>
            </div>

            {/* Filters and Sort Section - Minimalist Design */}
            <div className="flex flex-wrap justify-center gap-4">
              {/* Filter by Language */}
              <Select value={language} onValueChange={setLanguage}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <Filter className="h-5 w-5 text-[#515b65]" />{" "}
                  {/* Changed icon color */}
                  <SelectValue placeholder="Language" />
                </SelectTrigger>
                <SelectContent className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
                  <SelectItem
                    value="all"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    All Languages
                  </SelectItem>
                  <SelectItem
                    value="javascript"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    JavaScript
                  </SelectItem>
                  <SelectItem
                    value="typescript"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    TypeScript
                  </SelectItem>
                  <SelectItem
                    value="python"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Python
                  </SelectItem>
                  <SelectItem
                    value="java"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Java
                  </SelectItem>
                  <SelectItem
                    value="go"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Go
                  </SelectItem>
                  <SelectItem
                    value="rust"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Rust
                  </SelectItem>
                  <SelectItem
                    value="cpp"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    C++
                  </SelectItem>
                </SelectContent>
              </Select>

              {/* Filter by Topic */}
              <Select value={selectedTopic} onValueChange={setSelectedTopic}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <Filter className="h-5 w-5 text-[#515b65]" />{" "}
                  {/* Changed icon color */}
                  <SelectValue placeholder="Topic" />
                </SelectTrigger>
                <SelectContent className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
                  <SelectItem
                    value="all"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    All Topics
                  </SelectItem>
                  <SelectItem
                    value="frontend"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Frontend
                  </SelectItem>
                  <SelectItem
                    value="backend"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Backend
                  </SelectItem>
                  <SelectItem
                    value="devops"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    DevOps
                  </SelectItem>
                  <SelectItem
                    value="mobile"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Mobile
                  </SelectItem>
                  <SelectItem
                    value="ai"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    AI / ML
                  </SelectItem>
                </SelectContent>
              </Select>

              {/* Sort By */}
              <Select value={sortBy} onValueChange={setSortBy}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <ArrowDownUp className="h-5 w-5 text-[#515b65]" />{" "}
                  {/* Changed icon color */}
                  <SelectValue placeholder="Sort by" />
                </SelectTrigger>
                <SelectContent className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
                  <SelectItem
                    value="stars"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Stars
                  </SelectItem>
                  <SelectItem
                    value="forks"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Forks
                  </SelectItem>
                  <SelectItem
                    value="updated"
                    className="text-[#f3f3f3] hover:bg-[#f3c9a4]/10 focus:bg-[#f3c9a4]/10 focus:text-[#f3f3f3]"
                  >
                    Recently Updated
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </form>
        </div>

        {/* Repository Results */}
        <div className="max-w-7xl mx-auto">
          {loading ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
              {Array.from({ length: 6 }).map((_, i) => (
                <Card
                  key={i}
                  className="bg-[#292f36] border-[#515b65] rounded-lg shadow-md"
                >
                  <CardHeader className="p-6">
                    <Skeleton className="h-6 w-3/4 bg-[#515b65] rounded" />
                    <Skeleton className="h-4 w-full bg-[#515b65] rounded" />
                  </CardHeader>
                  <CardContent className="p-6">
                    <Skeleton className="h-4 w-full mb-3 bg-[#515b65] rounded" />
                    <Skeleton className="h-4 w-2/3 bg-[#515b65] rounded" />
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : showExamples ? (
            <div>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                {/* Example repositories will be fetched from the backend */}
              </div>
            </div>
          ) : repositories.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
              {repositories.map((repo, index) => (
                <div key={repo._id || `${repo.full_name}-${index}`}>
                  <Link
                    href={`/repo/${repo.full_name}`}
                    className="block transition-all duration-300 hover:scale-105"
                  >
                    <Card className="h-full bg-[#292f36] border-[#515b65] rounded-lg shadow-md hover:shadow-lg hover:bg-[#292f36]/90 transition-all duration-300">
                      <CardHeader className="p-6">
                        <div className="flex items-center gap-3 mb-3">
                          <img
                            src={repo.image_url || "/placeholder.svg"}
                            alt={repo.owner}
                            className="w-8 h-8 rounded-full"
                          />
                          <span className="text-sm text-[#f3f3f3]/70 font-medium">
                            {repo.owner}
                          </span>
                        </div>
                        <CardTitle className="text-xl text-[#f3f3f3] font-semibold">
                          {repo.name}
                        </CardTitle>
                        <CardDescription className="line-clamp-2 min-h-[3rem] text-[#f3f3f3]/60 leading-relaxed">
                          {repo.description || "No description available"}
                        </CardDescription>
                      </CardHeader>
                      <CardContent className="p-6 pt-0 space-y-4">
                        <div className="flex flex-wrap gap-4 items-center">
                          <div className="flex items-center gap-2 text-[#f3f3f3]/80 text-xs">
                            <Star className="h-4 w-4 text-[#f1e05a]" />
                            <span className="font-medium">
                              {repo.stargazers_count.toLocaleString()} stars
                            </span>
                          </div>
                          <div className="flex items-center gap-2 text-[#f3f3f3]/80 text-xs">
                            <GitFork className="h-4 w-4" />
                            <span className="font-medium">
                              {repo.forks_count.toLocaleString()} forks
                            </span>
                          </div>
                          <div className="flex items-center gap-2 text-[#f3f3f3]/80 text-xs">
                            <Eye className="h-4 w-4" />
                            <span className="font-medium">
                              {repo.watchers_count.toLocaleString()} watching
                            </span>
                          </div>
                          <div className="flex items-center gap-2 text-[#f3f3f3]/80 text-xs">
                            <AlertCircle className="h-4 w-4" />
                            <span className="font-medium">
                              {repo.open_issues_count} open issues
                            </span>
                          </div>
                          {repo.language && (
                            <Badge
                              variant="outline"
                              className="px-3 py-1 rounded-md font-medium bg-[#515b65]/20 border-[#515b65] text-[#f3f3f3]"
                            >
                              {repo.language}
                            </Badge>
                          )}
                        </div>

                        {repo.topics && repo.topics.length > 0 && (
                          <div className="flex flex-wrap gap-2">
                            {repo.topics.slice(0, 2).map((topic) => (
                              <Badge
                                key={topic}
                                variant="outline"
                                className="border-[#515b65] text-[#f3f3f3]/70 px-3 py-1 rounded-md hover:border-[#f3c9a4]/50 hover:text-[#f3c9a4] transition-colors duration-200"
                              >
                                {topic}
                              </Badge>
                            ))}
                          </div>
                        )}

                        <p className="text-xs text-[#f3f3f3]/50 font-medium">
                          Updated{" "}
                          {new Date(repo.pushed_at).toLocaleDateString()}
                        </p>
                      </CardContent>
                    </Card>
                  </Link>
                </div>
              ))}
            </div>
          ) : searchQuery && !loading ? (
            <div className="text-center py-16">
              <p className="text-[#f3f3f3]/70 text-lg mb-6">
                No repositories found for "{searchQuery}"
              </p>
              <Button
                variant="outline"
                className="bg-transparent border border-[#f3c9a4] text-[#f3c9a4] hover:bg-[#f3c9a4]/10 active:bg-[#f3c9a4]/20 rounded-lg px-6 py-3 font-medium transition-all duration-200"
                onClick={() => {
                  setSearchQuery("");
                  setRepositories([]);
                  setShowExamples(true);
                }}
              >
                View Examples
              </Button>
            </div>
          ) : (
            <div className="text-center py-16">
              <p className="text-[#f3f3f3]/70 text-lg">
                Search for repositories to get started
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
