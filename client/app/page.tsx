"use client"

import type React from "react"

import { useState, useEffect } from "react"
import { Search, Star, GitFork, Eye, Filter, ArrowDownUp } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import Link from "next/link"

interface Repository {
  id: number
  name: string
  full_name: string
  description: string
  stargazers_count: number
  forks_count: number
  watchers_count: number
  language: string
  topics: string[]
  owner: {
    login: string
    avatar_url: string
  }
  updated_at: string
}

export default function HomePage() {
  const [searchQuery, setSearchQuery] = useState("")
  const [repositories, setRepositories] = useState<Repository[]>([])
  const [loading, setLoading] = useState(false)
  const [sortBy, setSortBy] = useState("stars")
  const [language, setLanguage] = useState("all")
  const [selectedTopic, setSelectedTopic] = useState("all")

  const [showExamples, setShowExamples] = useState(true)

  const exampleRepositories: Repository[] = [
    {
      id: 1,
      name: "react",
      full_name: "facebook/react",
      description: "The library for web and native user interfaces.",
      stargazers_count: 228000,
      forks_count: 46800,
      watchers_count: 228000,
      language: "JavaScript",
      topics: ["react", "javascript", "library", "ui", "frontend"],
      owner: {
        login: "facebook",
        avatar_url: "https://avatars.githubusercontent.com/u/69631?v=4",
      },
      updated_at: "2024-12-01T10:30:00Z",
    },
    {
      id: 2,
      name: "next.js",
      full_name: "vercel/next.js",
      description: "The React Framework for the Web",
      stargazers_count: 125000,
      forks_count: 26800,
      watchers_count: 125000,
      language: "TypeScript",
      topics: ["react", "nextjs", "framework", "typescript", "vercel"],
      owner: {
        login: "vercel",
        avatar_url: "https://avatars.githubusercontent.com/u/14985020?v=4",
      },
      updated_at: "2024-12-01T09:15:00Z",
    },
    {
      id: 3,
      name: "vue",
      full_name: "vuejs/vue",
      description: "Vue.js is a progressive, incrementally-adoptable JavaScript framework for building UI on the web.",
      stargazers_count: 207000,
      forks_count: 33700,
      watchers_count: 207000,
      language: "TypeScript",
      topics: ["vue", "javascript", "framework", "frontend", "spa"],
      owner: {
        login: "vuejs",
        avatar_url: "https://avatars.githubusercontent.com/u/6128107?v=4",
      },
      updated_at: "2024-11-30T14:20:00Z",
    },
    {
      id: 4,
      name: "tensorflow",
      full_name: "tensorflow/tensorflow",
      description: "An Open Source Machine Learning Framework for Everyone",
      stargazers_count: 185000,
      forks_count: 74200,
      watchers_count: 185000,
      language: "C++",
      topics: ["tensorflow", "machine-learning", "deep-learning", "neural-network", "ai"],
      owner: {
        login: "tensorflow",
        avatar_url: "https://avatars.githubusercontent.com/u/15658638?v=4",
      },
      updated_at: "2024-12-01T08:45:00Z",
    },
    {
      id: 5,
      name: "vscode",
      full_name: "microsoft/vscode",
      description: "Visual Studio Code",
      stargazers_count: 163000,
      forks_count: 28900,
      watchers_count: 163000,
      language: "TypeScript",
      topics: ["vscode", "editor", "typescript", "electron", "ide"],
      owner: {
        login: "microsoft",
        avatar_url: "https://avatars.githubusercontent.com/u/6154722?v=4",
      },
      updated_at: "2024-12-01T11:10:00Z",
    },
    {
      id: 6,
      name: "node",
      full_name: "nodejs/node",
      description: "Node.js JavaScript runtime",
      stargazers_count: 107000,
      forks_count: 29200,
      watchers_count: 107000,
      language: "JavaScript",
      topics: ["nodejs", "javascript", "runtime", "server", "backend"],
      owner: {
        login: "nodejs",
        avatar_url: "https://avatars.githubusercontent.com/u/9950313?v=4",
      },
      updated_at: "2024-12-01T07:30:00Z",
    },
  ]

  const searchRepositories = async () => {
    setShowExamples(false)
    if (!searchQuery.trim()) return

    setLoading(true)
    try {
      const languageFilter = language !== "all" ? `+language:${language}` : ""
      const topicFilter = selectedTopic !== "all" ? `+topic:${selectedTopic}` : ""
      const response = await fetch(
        `https://api.github.com/search/repositories?q=${encodeURIComponent(searchQuery)}${languageFilter}${topicFilter}&sort=${sortBy}&order=desc&per_page=20`,
      )
      const data = await response.json()
      setRepositories(data.items || [])
    } catch (error) {
      console.error("Error searching repositories:", error)
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    searchRepositories()
  }

  useEffect(() => {
    if (searchQuery.trim()) {
      const debounceTimer = setTimeout(() => {
        searchRepositories()
      }, 500)
      return () => clearTimeout(debounceTimer)
    }
  }, [sortBy, language, selectedTopic, searchQuery])

  return (
    <div className="min-h-screen bg-[#16191d]">
      <div className="container mx-auto px-6 py-8">
        <div className="text-center mb-12">
          <h1 className="text-5xl text-[#f3c9a4] font-bold mb-6 leading-relaxed py-2 font-oswald">
            GitHub Repository Explorer
          </h1>
          <p className="text-[#f3f3f3]/80 text-xl max-w-2xl mx-auto leading-relaxed">
            Discover and explore open source repositories with AI-powered issue guidance
          </p>
        </div>

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

            <div className="flex flex-wrap justify-center gap-4">
              <Select value={language} onValueChange={setLanguage}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <Filter className="h-5 w-5 text-[#515b65]" />
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

              <Select value={selectedTopic} onValueChange={setSelectedTopic}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <Filter className="h-5 w-5 text-[#515b65]" />
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

              <Select value={sortBy} onValueChange={setSortBy}>
                <SelectTrigger className="w-full md:w-[180px] h-12 bg-[#292f36] border-[#515b65] rounded-lg text-[#f3f3f3] flex items-center gap-2 pl-4">
                  <ArrowDownUp className="h-5 w-5 text-[#515b65]" />
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

        <div className="max-w-7xl mx-auto">
          {loading ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
              {Array.from({ length: 6 }).map((_, i) => (
                <Card key={i} className="bg-[#292f36] border-[#515b65] rounded-lg shadow-md">
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
                {exampleRepositories.map((repo) => (
                  <Link
                    key={repo.id}
                    href={`/repo/${repo.owner.login}/${repo.name}`}
                    className="block transition-all duration-300 hover:scale-105"
                  >
                    <Card className="h-full bg-[#292f36] border-[#515b65] rounded-lg shadow-md hover:shadow-lg hover:bg-[#292f36]/90 transition-all duration-300">
                      <CardHeader className="p-6">
                        <div className="flex items-center gap-3 mb-3">
                          <img
                            src={repo.owner.avatar_url || "/placeholder.svg"}
                            alt={repo.owner.login}
                            className="w-8 h-8 rounded-full"
                          />
                          <span className="text-sm text-[#f3f3f3]/70 font-medium">{repo.owner.login}</span>
                        </div>
                        <CardTitle className="text-xl text-[#f3f3f3] font-semibold">{repo.name}</CardTitle>
                        <CardDescription className="line-clamp-2 text-[#f3f3f3]/60 leading-relaxed">
                          {repo.description || "No description available"}
                        </CardDescription>
                      </CardHeader>
                      <CardContent className="p-6">
                        <div className="flex items-center gap-6 text-sm text-[#f3f3f3]/70 mb-4">
                          <div className="flex items-center gap-2">
                            <Star className="h-4 w-4 text-[#f1e05a]" />
                            <span className="font-medium">{repo.stargazers_count.toLocaleString()}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <GitFork className="h-4 w-4" />
                            <span className="font-medium">{repo.forks_count.toLocaleString()}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Eye className="h-4 w-4" />
                            <span className="font-medium">{repo.watchers_count.toLocaleString()}</span>
                          </div>
                        </div>

                        <div className="flex flex-wrap gap-2 mb-4">
                          {repo.language && (
                            <Badge
                              variant="outline"
                              className="border-[#f3c9a4] text-[#f3c9a4] px-3 py-1 rounded-md font-medium hover:bg-[#f3c9a4]/10 transition-colors duration-200"
                            >
                              {repo.language}
                            </Badge>
                          )}
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

                        <p className="text-xs text-[#f3f3f3]/50 font-medium">
                          Updated {new Date(repo.updated_at).toLocaleDateString()}
                        </p>
                      </CardContent>
                    </Card>
                  </Link>
                ))}
              </div>
            </div>
          ) : repositories.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
              {repositories.map((repo) => (
                <Link
                  key={repo.id}
                  href={`/repo/${repo.owner.login}/${repo.name}`}
                  className="block transition-all duration-300 hover:scale-105"
                >
                  <Card className="h-full bg-[#292f36] border-[#515b65] rounded-lg shadow-md hover:shadow-lg hover:bg-[#292f36]/90 transition-all duration-300">
                    <CardHeader className="p-6">
                      <div className="flex items-center gap-3 mb-3">
                        <img
                          src={repo.owner.avatar_url || "/placeholder.svg"}
                          alt={repo.owner.login}
                          className="w-8 h-8 rounded-full"
                        />
                        <span className="text-sm text-[#f3f3f3]/70 font-medium">{repo.owner.login}</span>
                      </div>
                      <CardTitle className="text-xl text-[#f3f3f3] font-semibold">{repo.name}</CardTitle>
                      <CardDescription className="line-clamp-2 text-[#f3f3f3]/60 leading-relaxed">
                        {repo.description || "No description available"}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="p-6">
                      <div className="flex items-center gap-6 text-sm text-[#f3f3f3]/70 mb-4">
                        <div className="flex items-center gap-2">
                          <Star className="h-4 w-4 text-[#f1e05a]" />
                          <span className="font-medium">{repo.stargazers_count.toLocaleString()}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <GitFork className="h-4 w-4" />
                          <span className="font-medium">{repo.forks_count.toLocaleString()}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <Eye className="h-4 w-4" />
                          <span className="font-medium">{repo.watchers_count.toLocaleString()}</span>
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-2 mb-4">
                        {repo.language && (
                          <Badge
                            variant="outline"
                            className="border-[#f3c9a4] text-[#f3c9a4] px-3 py-1 rounded-md font-medium hover:bg-[#f3c9a4]/10 transition-colors duration-200"
                          >
                            {repo.language}
                          </Badge>
                        )}
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

                      <p className="text-xs text-[#f3f3f3]/50 font-medium">
                        Updated {new Date(repo.updated_at).toLocaleDateString()}
                      </p>
                    </CardContent>
                  </Card>
                </Link>
              ))}
            </div>
          ) : searchQuery && !loading ? (
            <div className="text-center py-16">
              <p className="text-[#f3f3f3]/70 text-lg mb-6">No repositories found for "{searchQuery}"</p>
              <Button
                variant="outline"
                className="bg-transparent border border-[#f3c9a4] text-[#f3c9a4] hover:bg-[#f3c9a4]/10 active:bg-[#f3c9a4]/20 rounded-lg px-6 py-3 font-medium transition-all duration-200"
                onClick={() => {
                  setSearchQuery("")
                  setRepositories([])
                  setShowExamples(true)
                }}
              >
                View Examples
              </Button>
            </div>
          ) : (
            <div className="text-center py-16">
              <p className="text-[#f3f3f3]/70 text-lg">Search for repositories to get started</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
