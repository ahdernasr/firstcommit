"use client"

import { useState, useEffect } from "react"
import { useParams } from "next/navigation"
import { Star, GitFork, Eye, ExternalLink, MessageSquare, AlertCircle, Calendar, Filter, Sparkles } from "lucide-react" // Added Sparkles icon
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import Link from "next/link"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm" // Corrected import path
import { getApiEndpoint } from '@/lib/config'

interface RepoSDetail {
  repo: Repository
  issues: Issue[]
}

interface Repository {
  id: string
  owner: string
  name: string
  full_name: string
  description: string
  stargazers_count: number
  watchers_count: number
  forks_count: number
  open_issues_count: number
  license: string
  homepage: string
  default_branch: string
  created_at: string
  pushed_at: string
  size: number
  visibility: string
  archived: boolean
  allow_forking: boolean
  is_template: boolean
  topics: string[]
  languages: string[]
  image_url: string
  readme?: string
  score: number
}

interface Issue {
  id: number
  number: number
  title: string
  body?: string // Make optional
  state: string
  user: {
    login: string
    avatar_url: string
  }
  labels: Array<{
    name: string
    color: string
  }>
  created_at: string
  comments: number
}

// Helper function to get API URL
const getApiUrl = () => {
  // Hardcode the backend URL for now
  const apiUrl = 'https://backend-222198140851.us-central1.run.app'
  console.log('Using API URL:', apiUrl)
  return apiUrl
}

export default function RepoPage() {
  const params = useParams()
  const [repository, setRepository] = useState<Repository | null>(null)
  const [issues, setIssues] = useState<Issue[]>([])
  const [loading, setLoading] = useState(true)
  const [issuesLoading, setIssuesLoading] = useState(true)
  const [currentPage, setCurrentPage] = useState(1)
  const [issuesPerPage] = useState(5)
  const [selectedIssueState, setSelectedIssueState] = useState<"open" | "closed">("open")
  const [selectedIssueLabel, setSelectedIssueLabel] = useState<string>("all")
  const [selectedSort, setSelectedSort] = useState<"created" | "updated" | "comments">("created")
  const [error, setError] = useState<string | null>(null)

  const fetchIssues = async (issueState: "open" | "closed", issueLabel: string, sortBy: string) => {
    setIssuesLoading(true)
    try {
      let url = `https://api.github.com/repos/${params.owner}/${params.name}/issues?per_page=100`
      url += `&state=${issueState}`
      url += `&sort=${sortBy}`
      url += `&direction=desc`
      if (issueLabel !== "all") {
        url += `&labels=${encodeURIComponent(issueLabel)}`
      }

      const response = await fetch(url)
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      const data = await response.json()

      // Filter out pull requests
      const onlyIssues = data.filter((item: any) => !item.pull_request)

      setIssues(onlyIssues)
    } catch (error) {
      console.error("Error fetching issues:", error)
      setIssues([])
    } finally {
      setIssuesLoading(false)
    }
  }

  const fetchData = async () => {
    setLoading(true)
    try {
      const fullUrl = getApiEndpoint(`/api/v1/repos/${params.owner}/${params.name}`)
      console.log('Fetching repo data from:', fullUrl)
      
      const response = await fetch(fullUrl, {
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('Repo API error:', {
          status: response.status,
          statusText: response.statusText,
          error: errorText
        })
        throw new Error(`HTTP error! status: ${response.status}, message: ${errorText}`)
      }

      const data: RepoSDetail = await response.json()
      console.log('Received repository data:', data)
      setRepository(data.repo)
    } catch (error) {
      console.error("Error fetching repository:", error)
      setError("Failed to load repository data")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    fetchIssues(selectedIssueState, selectedIssueLabel, selectedSort)
    setCurrentPage(1) // Reset pagination when filter changes
  }, [params.owner, params.name, selectedIssueState, selectedIssueLabel, selectedSort])

  const paginate = (pageNumber: number) => {
    setCurrentPage(pageNumber)
    // Scroll back to top of card header when changing pages
    const cardHeader = document.querySelector(".lg\\:col-span-2 .bg-\\[\\#292f36\\]")
    if (cardHeader) {
      cardHeader.scrollIntoView({ behavior: "smooth", block: "start" })
    } else {
      // Fallback to issues-list if card not found
      window.scrollTo({ top: document.getElementById("issues-list")?.offsetTop || 0, behavior: "smooth" })
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-[#16191d]">
        <div className="container mx-auto px-6 py-8">
          <Skeleton className="h-10 w-64 mb-6 bg-[#515b65] rounded-lg" />
          <Skeleton className="h-6 w-96 mb-12 bg-[#515b65] rounded-lg" />
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2">
              <Skeleton className="h-80 w-full bg-[#515b65] rounded-lg" />
            </div>
            <div>
              <Skeleton className="h-64 w-full bg-[#515b65] rounded-lg" />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!repository) {
    return (
      <div className="min-h-screen bg-[#16191d] flex items-center justify-center">
        <p className="text-[#f3f3f3]/70 text-lg">Repository not found</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[#16191d]">
      <div className="container mx-auto px-6 py-8">
        {/* Repository Header */}
        <div className="mb-12">
          <div className="flex items-center gap-4 mb-6">
            <img
              src={repository.image_url || "/placeholder.svg"}
              alt={repository.owner}
              className="w-16 h-16 rounded-full shadow-md"
            />
            <div>
              <h1 className="text-4xl font-bold text-[#f3f3f3] mb-2">{repository.full_name || `${repository.owner}/${repository.name}`}</h1>
              <p className="text-[#f3f3f3]/70 text-lg leading-relaxed">{repository.description || 'No description available'}</p>
            </div>
          </div>

          <div className="flex flex-wrap gap-6 items-center mb-6">
            <div className="flex items-center gap-2 text-[#f3f3f3]/80">
              <Star className="h-5 w-5 text-[#f1e05a]" />
              <span className="font-medium">{repository.stargazers_count || 0} stars</span>
            </div>
            <div className="flex items-center gap-2 text-[#f3f3f3]/80">
              <GitFork className="h-5 w-5" />
              <span className="font-medium">{repository.forks_count || 0} forks</span>
            </div>
            <div className="flex items-center gap-2 text-[#f3f3f3]/80">
              <Eye className="h-5 w-5" />
              <span className="font-medium">{repository.watchers_count || 0} watching</span>
            </div>
            <div className="flex items-center gap-2 text-[#f3f3f3]/80">
              <MessageSquare className="h-5 w-5" />
              <span className="font-medium">{repository.open_issues_count || 0} issues</span>
            </div>
            <div className="flex items-center gap-2 text-[#f3f3f3]/70">
              <Calendar className="h-5 w-5" />
              <span className="font-medium">Updated {repository.pushed_at ? new Date(repository.pushed_at).toLocaleDateString() : 'N/A'}</span>
            </div>
          </div>

          <div className="flex flex-wrap gap-3 mb-6">
            {repository.languages && repository.languages.length > 0 && (
              <Badge className="bg-[#0b84ff] text-[#16191d] px-4 py-2 rounded-lg font-medium hover:bg-[#0066cc] transition-colors duration-200">
                {repository.languages[0]}
              </Badge>
            )}
            {repository.license && (
              <Badge variant="outline" className="border-[#515b65] text-[#f3f3f3]/70 px-4 py-2 rounded-lg">
                {repository.license}
              </Badge>
            )}
            {repository.topics && repository.topics.map((topic) => (
              <Badge
                key={topic}
                variant="outline"
                className="border-[#515b65] text-[#f3f3f3]/70 px-4 py-2 rounded-lg hover:border-[#0b84ff]/50 hover:text-[#0b84ff] transition-colors duration-200"
              >
                {topic}
              </Badge>
            ))}
          </div>

          <div className="flex gap-4">
            <Button
              asChild
              className="bg-transparent border border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/10 active:bg-[#0b84ff]/20 px-6 py-3 rounded-lg font-medium shadow-md hover:shadow-lg transition-all duration-200"
            >
              <a href={repository.homepage} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="h-5 w-5 mr-2" />
                View on GitHub
              </a>
            </Button>
          </div>
        </div>

        <Separator className="mb-12 bg-[#515b65]" />

        {/* Issue Filters and Issues */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Issue Filters and Sorting */}
          <div className="lg:col-span-1">
            <Card className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
              <CardHeader className="p-6">
                <CardTitle className="text-xl text-[#f3f3f3] flex items-center gap-2">
                  <Filter className="h-5 w-5" />
                  Filters & Sort
                </CardTitle>
                <CardDescription className="text-[#f3f3f3]/60">
                  Filter and sort issues to find what you need
                </CardDescription>
              </CardHeader>
              <CardContent className="p-6 space-y-6">
                {/* Issue State Filter */}
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-[#f3f3f3] uppercase tracking-wide flex items-center gap-2">
                    <div className="w-1 h-4 bg-[#0b84ff] rounded-full"></div>
                    Issue State
                  </h3>
                  <div className="grid grid-cols-1 gap-2">
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueState("open")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueState === "open"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      Open Issues
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueState("closed")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueState === "closed"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      Closed Issues
                    </Button>
                  </div>
                </div>

                <Separator className="bg-[#515b65]" />

                {/* Label Filters */}
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-[#f3f3f3] uppercase tracking-wide flex items-center gap-2">
                    <div className="w-1 h-4 bg-[#0b84ff] rounded-full"></div>
                    Filter by Label
                  </h3>
                  <div className="grid grid-cols-1 gap-2">
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("all")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "all"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      All Labels
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("good first issue")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "good first issue"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                        Good First Issue
                      </div>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("help wanted")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "help wanted"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-blue-500 rounded-full"></div>
                        Help Wanted
                      </div>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("bug")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "bug"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-red-500 rounded-full"></div>
                        Bug Reports
                      </div>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("enhancement")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "enhancement"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-purple-500 rounded-full"></div>
                        Enhancements
                      </div>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedIssueLabel("documentation")}
                      className={`w-full justify-start text-sm ${
                        selectedIssueLabel === "documentation"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-yellow-500 rounded-full"></div>
                        Documentation
                      </div>
                    </Button>
                  </div>
                </div>

                <Separator className="bg-[#515b65]" />

                {/* Sort Options */}
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-[#f3f3f3] uppercase tracking-wide flex items-center gap-2">
                    <div className="w-1 h-4 bg-[#0b84ff] rounded-full"></div>
                    Sort By
                  </h3>
                  <div className="grid grid-cols-1 gap-2">
                    <Button
                      variant="outline"
                      onClick={() => setSelectedSort("created")}
                      className={`w-full justify-start text-sm ${
                        selectedSort === "created"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      Newest First
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedSort("updated")}
                      className={`w-full justify-start text-sm ${
                        selectedSort === "updated"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      Recently Updated
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedSort("comments")}
                      className={`w-full justify-start text-sm ${
                        selectedSort === "comments"
                          ? "bg-[#0b84ff]/10 border-[#0b84ff] text-[#0b84ff] hover:bg-[#0b84ff]/20"
                          : "bg-transparent border-[#515b65] text-[#f3f3f3]/60 hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                      }`}
                    >
                      Most Comments
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Issues List */}
          <div className="lg:col-span-2">
            <Card className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
              <CardHeader className="p-6">
                <CardTitle className="flex items-center gap-3 text-xl text-[#f3f3f3]">
                  <MessageSquare className="h-6 w-6" />
                  {selectedIssueState === "open" ? "Open Issues" : "Closed Issues"}
                  {selectedIssueLabel !== "all" && (
                    <Badge variant="outline" className="border-[#0b84ff] text-[#0b84ff] text-sm">
                      {selectedIssueLabel}
                    </Badge>
                  )}
                </CardTitle>
                <CardDescription className="text-[#f3f3f3]/60">
                  Click "Guide Me" on any issue to get AI-powered assistance
                </CardDescription>
              </CardHeader>
              <CardContent className="p-6">
                <div id="issues-list">
                  {issuesLoading ? (
                    <div className="space-y-6">
                      {Array.from({ length: 5 }).map((_, i) => (
                        <div key={i} className="border border-[#515b65] rounded-lg p-6">
                          <Skeleton className="h-6 w-3/4 mb-3 bg-[#515b65] rounded" />
                          <Skeleton className="h-4 w-full mb-3 bg-[#515b65] rounded" />
                          <Skeleton className="h-4 w-1/2 bg-[#515b65] rounded" />
                        </div>
                      ))}
                    </div>
                  ) : issues.length > 0 ? (
                    <>
                      <div className="space-y-6">
                        {/* Get current issues for pagination */}
                        {/* Calculate pagination values */}
                        {(() => {
                          const totalPages = Math.ceil(issues.length / issuesPerPage)
                          const startIndex = (currentPage - 1) * issuesPerPage
                          const endIndex = startIndex + issuesPerPage
                          const currentIssues = issues.slice(startIndex, endIndex)

                          return (
                            <>
                              {currentIssues.map((issue) => (
                                <div
                                  key={issue.id}
                                  className="border border-[#515b65] rounded-lg p-6 hover:bg-[#292f36]/80 hover:shadow-md transition-all duration-200"
                                >
                                  <div className="flex items-center justify-between mb-4">
                                    {/* Removed the "Closed" badge */}
                                    <h3 className="font-semibold text-lg text-[#f3f3f3] leading-tight flex-1 pr-4">
                                      #{issue.number} {issue.title}
                                    </h3>
                                    <Link href={`/repo/${params.owner}/${params.name}/issue/${issue.number}`}>
                                      <Button className="bg-[#0b84ff] text-[#16191d] px-4 py-2 rounded-lg font-medium shadow-md hover:shadow-lg transition-all duration-200">
                                        Guide Me
                                      </Button>
                                    </Link>
                                  </div>

                                  <div className="flex items-center gap-4 mb-4">
                                    <img
                                      src={issue.user.avatar_url || "/placeholder.svg"}
                                      alt={issue.user.login}
                                      className="w-6 h-6 rounded-full"
                                    />
                                    <span className="text-sm text-[#f3f3f3]/70 font-medium">by {issue.user.login}</span>
                                    <span className="text-sm text-[#f3f3f3]/60">
                                      {new Date(issue.created_at).toLocaleDateString()}
                                    </span>
                                    {issue.comments > 0 && (
                                      <span className="text-sm text-[#f3f3f3]/60 flex items-center gap-1">
                                        <MessageSquare className="h-4 w-4" />
                                        {issue.comments}
                                      </span>
                                    )}
                                  </div>

                                  {issue.labels.length > 0 && (
                                    <div className="flex flex-wrap gap-2 mb-4">
                                      {issue.labels.map((label) => (
                                        <Badge
                                          key={label.name}
                                          variant="outline"
                                          className="px-3 py-1 rounded-md text-xs font-medium"
                                          style={{
                                            backgroundColor: `#${label.color}20`,
                                            borderColor: `#${label.color}`,
                                            color: `#${label.color}`,
                                          }}
                                        >
                                          {label.name}
                                        </Badge>
                                      ))}
                                    </div>
                                  )}

                                  {issue.body && (
                                    <div className="text-sm text-[#f3f3f3]/70 line-clamp-2 leading-relaxed overflow-hidden">
                                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                        {issue.body.substring(0, 200) + "..."}
                                      </ReactMarkdown>
                                    </div>
                                  )}
                                </div>
                              ))}

                              {/* Pagination Controls */}
                              {issues.length > issuesPerPage && (
                                <div className="flex justify-center mt-8">
                                  <div className="flex items-center gap-2">
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      onClick={() => currentPage > 1 && paginate(currentPage - 1)}
                                      disabled={currentPage === 1}
                                      className="border-[#515b65] text-[#f3f3f3] hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff] disabled:opacity-50"
                                    >
                                      Previous
                                    </Button>

                                    {(() => {
                                      const totalPages = Math.ceil(issues.length / issuesPerPage)
                                      const maxVisiblePages = 5
                                      const halfVisible = Math.floor(maxVisiblePages / 2)
                                      let startPage = Math.max(1, currentPage - halfVisible)
                                      const endPage = Math.min(totalPages, startPage + maxVisiblePages - 1)

                                      // Adjust start page if we're near the end
                                      if (endPage - startPage + 1 < maxVisiblePages) {
                                        startPage = Math.max(1, endPage - maxVisiblePages + 1)
                                      }

                                      const pages = []

                                      // Show first page and ellipsis if needed
                                      if (startPage > 1) {
                                        pages.push(
                                          <Button
                                            key={1}
                                            variant="outline"
                                            size="sm"
                                            onClick={() => paginate(1)}
                                            className="border-[#515b65] text-[#f3f3f3] hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                                          >
                                            1
                                          </Button>,
                                        )
                                        if (startPage > 2) {
                                          pages.push(
                                            <span key="start-ellipsis" className="text-[#f3f3f3]/60 px-2">
                                              ...
                                            </span>,
                                          )
                                        }
                                      }

                                      // Show visible page range
                                      for (let i = startPage; i <= endPage; i++) {
                                        pages.push(
                                          <Button
                                            key={i}
                                            variant={currentPage === i ? "default" : "outline"}
                                            size="sm"
                                            onClick={() => paginate(i)}
                                            className={
                                              currentPage === i
                                                ? "bg-[#0b84ff] text-[#16191d]"
                                                : "border-[#515b65] text-[#f3f3f3] hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                                            }
                                          >
                                            {i}
                                          </Button>,
                                        )
                                      }

                                      // Show ellipsis and last page if needed
                                      if (endPage < totalPages) {
                                        if (endPage < totalPages - 1) {
                                          pages.push(
                                            <span key="end-ellipsis" className="text-[#f3f3f3]/60 px-2">
                                              ...
                                            </span>,
                                          )
                                        }
                                        pages.push(
                                          <Button
                                            key={totalPages}
                                            variant="outline"
                                            size="sm"
                                            onClick={() => paginate(totalPages)}
                                            className="border-[#515b65] text-[#f3f3f3] hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff]"
                                          >
                                            {totalPages}
                                          </Button>,
                                        )
                                      }

                                      return pages
                                    })()}

                                    <Button
                                      variant="outline"
                                      size="sm"
                                      onClick={() => currentPage < totalPages && paginate(currentPage + 1)}
                                      disabled={currentPage === totalPages}
                                      className="border-[#515b65] text-[#f3f3f3] hover:bg-[#0b84ff]/10 hover:border-[#0b84ff] hover:text-[#0b84ff] disabled:opacity-50"
                                    >
                                      Next
                                    </Button>
                                  </div>
                                </div>
                              )}
                            </>
                          )
                        })()}
                      </div>
                    </>
                  ) : (
                    <p className="text-[#f3f3f3]/60 text-center py-12 text-lg">
                      No open issues found for this repository.
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}
