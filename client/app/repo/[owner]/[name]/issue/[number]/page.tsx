"use client"

import type React from "react"
import { useState, useEffect, useRef } from "react"
import { useParams } from "next/navigation"
import { ArrowLeft, Calendar, MessageSquare } from "lucide-react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import Link from "next/link"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeRaw from "rehype-raw"
import rehypeSanitize from "rehype-sanitize"
import SyntaxHighlighter from "react-syntax-highlighter/dist/esm/prism"
import { tomorrow } from "react-syntax-highlighter/dist/esm/styles/prism"
import { ArrowUp } from "lucide-react"
import { getApiEndpoint } from '@/lib/config'
import FileLink from '@/app/components/FileLink'

interface Issue {
  id: number
  number: number
  title: string
  body?: string
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
  html_url: string
}

interface Comment {
  id: number
  body: string
  user: {
    login: string
    avatar_url: string
  }
  created_at: string
}

interface Message {
  id: string
  role: "user" | "assistant"
  content: string
}

// Add the helper function at the top level
function fixLooseMarkdown(md: string): string {
  return md
    .replace(/([^\n])\n(?=- )/g, '$1\n\n')       // ensure newline before `-` bullets
    .replace(/^(-)(\S)/gm, '$1 $2')              // ensure space after `-`
    .replace(/([^\n])\n(?=#+ )/g, '$1\n\n')      // ensure newline before headings
}

export default function IssuePage() {
  const params = useParams()
  const [issue, setIssue] = useState<Issue | null>(null)
  const [loading, setLoading] = useState(true)
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState("")
  const [isGenerating, setIsGenerating] = useState(false)
  const [comments, setComments] = useState<Comment[]>([])
  const [commentsLoading, setCommentsLoading] = useState(false)
  const [displayedContent, setDisplayedContent] = useState<{ [key: string]: string }>({})
  const [shouldAutoScroll, setShouldAutoScroll] = useState(true)
  const [guide, setGuide] = useState<string>("")
  const [guideLoading, setGuideLoading] = useState(true)
  const [isTyping, setIsTyping] = useState(false)
  const [loadingMessage, setLoadingMessage] = useState("")
  const [loadingStep, setLoadingStep] = useState(0)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  const loadingSteps = [
    "Analyzing issue context and requirements...",
    "Searching codebase for relevant implementations...",
    "Identifying key components and dependencies...",
    "Crafting comprehensive contribution guide..."
  ]

  // Scroll to bottom whenever messages change
  const scrollToBottom = () => {
    if (shouldAutoScroll) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
    }
  }
  // Scroll to bottom whenever messages or displayed content changes
  useEffect(() => {
    scrollToBottom()
  }, [messages, displayedContent])

  // Fetch issue data
  useEffect(() => {
    const fetchIssue = async () => {
      try {
        const response = await fetch(
          `https://api.github.com/repos/${params.owner}/${params.name}/issues/${params.number}`,
        )
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`)
        }
        const data = await response.json()
        setIssue(data)
        setMessages([]) // reset conversation on new issue
      } catch (error) {
        console.error("Error fetching issue:", error)
      } finally {
        setLoading(false)
      }
    }
    fetchIssue()
  }, [params.owner, params.name, params.number])

  // Fetch comments when dialog is opened
  const fetchComments = async () => {
    setCommentsLoading(true)
    try {
      const response = await fetch(
        `https://api.github.com/repos/${params.owner}/${params.name}/issues/${params.number}/comments`,
      )
      const data = await response.json()
      setComments(data)
    } catch (error) {
      console.error("Error fetching comments:", error)
    } finally {
      setCommentsLoading(false)
    }
  }

  // Fetch guide when issue loads
  useEffect(() => {
    const fetchGuide = async () => {
      if (!issue) return
      
      setGuideLoading(true)
      try {
        const fullUrl = getApiEndpoint('/api/v1/guide')
        console.log('Fetching guide from:', fullUrl)
        
        const response = await fetch(fullUrl, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            query: issue.title + "\n\n" + issue.body,
            repo_id: `${params.owner}/${params.name}`,
            issue_number: params.number,
          }),
        })

        if (!response.ok) {
          const errorText = await response.text()
          console.error('Guide API error:', {
            status: response.status,
            statusText: response.statusText,
            error: errorText
          })
          throw new Error(`HTTP error! status: ${response.status}, message: ${errorText}`)
        }

        const data = await response.json()
        setGuide(data.guide || "")
      } catch (error) {
        console.error("Error fetching guide:", error)
        setGuide("Failed to generate guide. Please try again later.")
      } finally {
        setGuideLoading(false)
      }
    }

    fetchGuide()
  }, [issue, params.owner, params.name, params.number])

  // Handle loading message typing animation
  useEffect(() => {
    if (!guideLoading) return

    const currentMessage = loadingSteps[loadingStep]
    let currentText = ""
    let charIndex = 0

    const typeInterval = setInterval(() => {
      if (charIndex < currentMessage.length) {
        currentText += currentMessage[charIndex]
        setLoadingMessage(currentText)
        charIndex++
      } else {
        clearInterval(typeInterval)
        // Move to next step after 5 seconds
        setTimeout(() => {
          if (loadingStep < loadingSteps.length - 1) {
            setLoadingStep(prev => prev + 1)
          }
        }, 5000)
      }
    }, 30) // Typing speed

    return () => clearInterval(typeInterval)
  }, [guideLoading, loadingStep])

  // Reset loading state when guide loads
  useEffect(() => {
    if (!guideLoading) {
      setLoadingStep(0)
      setLoadingMessage("")
    }
  }, [guideLoading])

  // Typing-effect: reveal one character at a time (much faster: 5ms)
  useEffect(() => {
    const lastMessage = messages[messages.length - 1]
    if (!lastMessage || lastMessage.role !== "assistant") {
      setIsTyping(false)
      return
    }

    const fullContent = lastMessage.content
    const id = lastMessage.id
    const already = displayedContent[id] ?? ""

    if (already.length < fullContent.length) {
      const timer = setTimeout(() => {
        setDisplayedContent((prev) => ({
          ...prev,
          [id]: fullContent.slice(0, already.length + 1),
        }))
        // Re-enable auto-scroll when new content is being generated
        setShouldAutoScroll(true)
        // Scroll after each character is added
        scrollToBottom()
      }, 1) // *** FAST TYPING: 1ms ***
      return () => clearTimeout(timer)
    } else {
      setIsTyping(false)
    }
  }, [messages, displayedContent])

  // Set isTyping when a new assistant message is added
  useEffect(() => {
    const lastMessage = messages[messages.length - 1]
    if (lastMessage?.role === "assistant") {
      setIsTyping(true)
    }
  }, [messages])

  const skipTyping = () => {
    const lastMessage = messages[messages.length - 1]
    if (!lastMessage || lastMessage.role !== "assistant") return

    setDisplayedContent((prev) => ({
      ...prev,
      [lastMessage.id]: lastMessage.content,
    }))
    setIsTyping(false)
  }

  // Detect when user scrolls away from bottom to disable auto-scroll
  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.pageYOffset || document.documentElement.scrollTop
      const windowHeight = window.innerHeight
      const documentHeight = document.documentElement.scrollHeight

      // Check if user is near the bottom (within 100px)
      const isNearBottom = scrollTop + windowHeight >= documentHeight - 100

      setShouldAutoScroll(isNearBottom)
    }

    window.addEventListener("scroll", handleScroll)
    return () => window.removeEventListener("scroll", handleScroll)
  }, [])

  // Handle form submit
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || isGenerating) return

    // 1) Add the user message immediately
    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content: input.trim(),
    }
    setMessages((prev) => [...prev, userMessage])
    setInput("")

    // Re-enable auto-scroll when user sends a message
    setShouldAutoScroll(true)

    // 2) Send message to backend
    setIsGenerating(true)
    try {
      const requestBody = {
        query: input.trim(),
        repo_id: `${params.owner}/${params.name}`,
      }
      console.log("Sending request:", requestBody)

      const fullUrl = getApiEndpoint('/api/v1/rag')
      console.log('Sending request to:', fullUrl)

      const response = await fetch(fullUrl, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(requestBody),
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('RAG API error:', {
          status: response.status,
          statusText: response.statusText,
          error: errorText
        })
        throw new Error(`HTTP error! status: ${response.status}, message: ${errorText}`)
      }

      const data = await response.json()
      console.log("Received response:", data)

      const assistantMessageId = (Date.now() + 1).toString()
      const assistantMessage: Message = {
        id: assistantMessageId,
        role: "assistant",
        content: data.answer || "No response from AI.",
      }
      setMessages((prev) => [...prev, assistantMessage])

      setDisplayedContent((prev) => ({
        ...prev,
        [assistantMessageId]: "",
      }))

    } catch (error) {
      console.error("Error sending chat message:", error)
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: error instanceof Error ? error.message : "Error communicating with AI. Please try again.",
      }
      setMessages((prev) => [...prev, errorMessage])
    } finally {
      setIsGenerating(false)
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-[#16191d]">
        <div className="container mx-auto px-6 py-8">
          <Skeleton className="h-10 w-64 mb-6 bg-[#515b65] rounded-lg" />
          <Skeleton className="h-80 w-full mb-12 bg-[#515b65] rounded-lg" />
          <Skeleton className="h-64 w-full bg-[#515b65] rounded-lg" />
        </div>
      </div>
    )
  }

  if (!issue) {
    return (
      <div className="min-h-screen bg-[#16191d] flex items-center justify-center">
        <p className="text-[#f3f3f3]/70 text-lg">Issue not found</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[#16191d]">
      <div className="container mx-auto px-6 py-8 max-w-5xl">
        {/* Back Button */}
        <div className="mb-8">
          <Link href={`/repo/${params.owner}/${params.name}`}>
            <Button
              variant="ghost"
              className="gap-3 bg-transparent text-[#f3c9a4] hover:bg-[#f3c9a4]/10 active:bg-[#f3c9a4]/20 rounded-lg px-4 py-3 font-medium transition-all duration-200"
            >
              <ArrowLeft className="h-5 w-5" />
              Back to Repository
            </Button>
          </Link>
        </div>

        {/* Issue Details */}
        <Card className="mb-12 bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
          <CardHeader className="p-6 pb-0">
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <CardTitle className="text-3xl mb-4 text-[#f3f3f3] font-bold leading-tight">
                  #{issue.number} {issue.title}
                </CardTitle>
                <CardDescription className="flex items-center gap-6 text-[#f3f3f3]/70">
                  <div className="flex items-center gap-3">
                    <img
                      src={issue.user.avatar_url || "/placeholder.svg"}
                      alt={issue.user.login}
                      className="w-10 h-10 rounded-full shadow-md"
                    />
                    <span className="font-medium">by {issue.user.login}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Calendar className="h-4 w-4" />
                    <span>{new Date(issue.created_at).toLocaleDateString()}</span>
                  </div>
                  {issue.comments > 0 && (
                    <Dialog>
                      <DialogTrigger asChild>
                        <button
                          className="flex items-center gap-2 text-sm text-[#f3f3f3]/70 hover:text-[#f3c9a4] transition-colors cursor-pointer"
                          onClick={fetchComments}
                        >
                          <MessageSquare className="h-4 w-4" />
                          <span>{issue.comments} comments</span>
                        </button>
                      </DialogTrigger>
                      <DialogContent className="max-w-2xl max-h-[80vh] bg-[#292f36] border-[#515b65]">
                        <DialogHeader>
                          <DialogTitle className="text-[#f3f3f3]">Comments ({issue.comments})</DialogTitle>
                        </DialogHeader>
                        <ScrollArea className="h-[60vh] pr-4 custom-scrollbar">
                          {commentsLoading ? (
                            <div className="space-y-4">
                              {Array.from({ length: 3 }).map((_, i) => (
                                <div key={i} className="border border-[#515b65] rounded-lg p-4">
                                  <Skeleton className="h-4 w-1/4 mb-2 bg-[#515b65]" />
                                  <Skeleton className="h-16 w-full bg-[#515b65]" />
                                </div>
                              ))}
                            </div>
                          ) : (
                            <div className="space-y-4">
                              {comments.map((comment) => (
                                <div key={comment.id} className="border border-[#515b65] rounded-lg p-4">
                                  <div className="flex items-center gap-3 mb-3">
                                    <img
                                      src={comment.user.avatar_url || "/placeholder.svg"}
                                      alt={comment.user.login}
                                      className="w-6 h-6 rounded-full"
                                    />
                                    <span className="font-medium text-[#f3f3f3]">{comment.user.login}</span>
                                    <span className="text-sm text-[#f3f3f3]/60">
                                      {new Date(comment.created_at).toLocaleDateString()}
                                    </span>
                                  </div>
                                  <div className="prose prose-md3 max-w-none">
                                    <ReactMarkdown
                                      remarkPlugins={[remarkGfm]}
                                      rehypePlugins={[rehypeRaw, rehypeSanitize]}
                                      components={{
                                        code({ inline, className, children, ...props }: any) {
                                          const match = /language-(\w+)/.exec(className || "")
                                          const isInline = inline && !match // Treat as inline if explicitly inline and not a code block

                                          if (isInline) {
                                            // Render inline code without SyntaxHighlighter
                                            return <code className={className} {...props}>{children}</code>
                                          }

                                          return match ? (
                                            <div className="relative group">
                                              <SyntaxHighlighter
                                                style={tomorrow}
                                                language={match[1]}
                                                PreTag="div"
                                                className="!mt-0 !mb-0"
                                                {...props}
                                              >
                                                {String(children).replace(/\n$/, "")}
                                              </SyntaxHighlighter>
                                              {/* Add tooltip for long file paths */}
                                              {match[1] === 'filepath' && (
                                                <div className="absolute inset-0 flex items-center">
                                                  <div className="w-full overflow-hidden text-ellipsis whitespace-nowrap px-4 py-2 text-sm text-[#f3f3f3]/70">
                                                    {children}
                                                  </div>
                                                  <div className="absolute inset-y-0 right-0 flex items-center">
                                                    <div className="h-full w-8 bg-gradient-to-l from-[#16191d] to-transparent" />
                                                  </div>
                                                </div>
                                              )}
                                            </div>
                                          ) : (
                                            <code className={className} {...props}>
                                              {children}
                                            </code>
                                          )
                                        },
                                        p: ({ children }) => (
                                          <p className="mb-2 leading-relaxed text-[#f3f3f3]">{children}</p>
                                        ),
                                      }}
                                    >
                                      {fixLooseMarkdown(comment.body)}
                                    </ReactMarkdown>
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </ScrollArea>
                      </DialogContent>
                    </Dialog>
                  )}
                </CardDescription>
              </div>
              <Badge
                variant={issue.state === "open" ? "default" : "secondary"}
                className={`px-4 py-2 rounded-lg font-medium cursor-default ${
                  issue.state === "open" ? "bg-[#f3c9a4] text-[#16191d]" : "bg-[#515b65] text-[#f3f3f3]"
                }`}
              >
                {issue.state}
              </Badge>
            </div>

            {issue.labels.length > 0 && (
              <div className="flex flex-wrap gap-3 mt-6">
                {issue.labels.map((label) => (
                  <Badge
                    key={label.name}
                    variant="outline"
                    className="px-3 py-1 rounded-md font-medium"
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
          </CardHeader>

          {issue.body && (
            <CardContent className="p-6 pt-3">
              <div className="prose prose-md3 max-w-none text-[#f3f3f3] leading-relaxed">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  rehypePlugins={[rehypeRaw, rehypeSanitize]}
                  components={{
                    code({ inline, className, children, ...props }: any) {
                      const match = /language-(\w+)/.exec(className || "")
                      const isInline = inline && !match // Treat as inline if explicitly inline and not a code block

                      if (isInline) {
                        // Render inline code without SyntaxHighlighter
                        return <code className={className} {...props}>{children}</code>
                      }

                      return match ? (
                        <div className="relative group">
                          <SyntaxHighlighter
                            style={tomorrow}
                            language={match[1]}
                            PreTag="div"
                            className="!mt-0 !mb-0"
                            {...props}
                          >
                            {String(children).replace(/\n$/, "")}
                          </SyntaxHighlighter>
                          {/* Add tooltip for long file paths */}
                          {match[1] === 'filepath' && (
                            <div className="absolute inset-0 flex items-center">
                              <div className="w-full overflow-hidden text-ellipsis whitespace-nowrap px-4 py-2 text-sm text-[#f3f3f3]/70">
                                {children}
                              </div>
                              <div className="absolute inset-y-0 right-0 flex items-center">
                                <div className="h-full w-8 bg-gradient-to-l from-[#16191d] to-transparent" />
                              </div>
                            </div>
                          )}
                        </div>
                      ) : (
                        <code className={className} {...props}>
                          {children}
                        </code>
                      )
                    },
                    pre: ({ children }) => (
                      <pre className="bg-[#16191d] border border-[#515b65] rounded-lg p-4 overflow-x-auto my-4">
                        {children}
                      </pre>
                    ),
                    h1: ({ children }) => (
                      <h1 className="text-2xl font-bold mb-4 text-[#f3f3f3] border-b border-[#515b65] pb-2">
                        {children}
                      </h1>
                    ),
                    h2: ({ children }) => (
                      <h2 className="text-xl font-semibold mb-3 text-[#f3f3f3] mt-6">{children}</h2>
                    ),
                    h3: ({ children }) => <h3 className="text-lg font-medium mb-2 text-[#f3f3f3] mt-4">{children}</h3>,
                    h4: ({ children }) => (
                      <h4 className="text-base font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h4>
                    ),
                    h5: ({ children }) => <h5 className="text-sm font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h5>,
                    h6: ({ children }) => <h6 className="text-sm font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h6>,
                    p: ({ children }) => <p className="mb-4 leading-relaxed text-[#f3f3f3]">{children}</p>,
                    ul: ({ children }) => (
                      <ul className="list-disc list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ul>
                    ),
                    ol: ({ children }) => (
                      <ol className="list-decimal list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ol>
                    ),
                    li: ({ children }) => <li className="text-[#f3f3f3]">{children}</li>,
                    blockquote: ({ children }) => (
                      <blockquote className="border-l-4 border-[#f3c9a4] pl-4 italic my-4 text-[#f3f3f3]/80 bg-[#f3c9a4]/5 py-2 rounded-r">
                        {children}
                      </blockquote>
                    ),
                    a: ({ href, children }) => {
                      // Check if the link is a file reference (e.g., src/file.txt)
                      if (href && !href.startsWith('http')) {
                        return (
                          <FileLink
                            repoId={`${params.owner}/${params.name}`}
                            filePath={href}
                          >
                            {children}
                          </FileLink>
                        );
                      }
                      // Regular external links
                      return (
                        <a
                          href={href}
                          className="text-[#f3c9a4] hover:underline"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {children}
                        </a>
                      );
                    },
                    table: ({ children }) => (
                      <div className="overflow-x-auto my-4">
                        <table className="min-w-full border border-[#515b65] rounded-lg">{children}</table>
                      </div>
                    ),
                    thead: ({ children }) => <thead className="bg-[#292f36]">{children}</thead>,
                    tbody: ({ children }) => <tbody>{children}</tbody>,
                    tr: ({ children }) => <tr className="border-b border-[#515b65]">{children}</tr>,
                    th: ({ children }) => (
                      <th className="border border-[#515b65] px-4 py-3 bg-[#292f36] font-semibold text-left text-[#f3f3f3]">
                        {children}
                      </th>
                    ),
                    td: ({ children }) => (
                      <td className="border border-[#515b65] px-4 py-3 text-[#f3f3f3]">{children}</td>
                    ),
                    strong: ({ children }) => <strong className="font-bold text-[#f3c9a4]">{children}</strong>,
                    em: ({ children }) => <em className="italic text-[#f3f3f3]/90">{children}</em>,
                    hr: () => <hr className="border-[#515b65] my-6" />,
                    img: ({ src, alt }) => (
                      <img
                        src={src || "/placeholder.svg"}
                        alt={alt || ""}
                        className="max-w-full h-auto rounded-lg border border-[#515b65] my-4"
                      />
                    ),
                  }}
                >
                  {(issue.body || "").replace(/<!--[\s\S]*?-->/g, "")}
                </ReactMarkdown>
              </div>
            </CardContent>
          )}
        </Card>

        <Separator className="mb-12 bg-[#515b65]" />

        {/* Guide Section */}
        <Card className="mb-12 bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
          <CardHeader className="p-6 pb-0">
            <CardTitle className="flex items-center gap-2 text-xl">
              <div className="relative h-6 w-6">
                <div
                  className="absolute inset-0 h-6 w-6"
                  style={{
                    background: "linear-gradient(-45deg, #f3c9a4, #3ac8bd, #f3c9a4, #3ac8bd)",
                    backgroundSize: "400% 400%",
                    animation: "gradient-x 6s ease infinite",
                    WebkitMask: "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'%3E%3Cpath d='M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z'/%3E%3Cpath d='M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z'/%3E%3C/svg%3E\") center/contain no-repeat",
                    mask: "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'%3E%3Cpath d='M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z'/%3E%3Cpath d='M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z'/%3E%3C/svg%3E\") center/contain no-repeat",
                  }}
                />
              </div>
              <span className="bg-gradient-to-r from-[#f3c9a4] to-[#3ac8bd] bg-clip-text text-transparent bg-size-200 animate-gradient-x font-semibold">
                First-Time Contributor Guide
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="p-6 pt-3">
            {guideLoading ? (
              <div className="space-y-6">
                <div className="flex items-center gap-3">
                  <div className="h-4 w-4 relative">
                    <div className="absolute inset-0 h-4 w-4 rounded-full border-2 border-[#f3c9a4] border-t-transparent animate-spin" />
                  </div>
                  <div className="text-[#f3f3f3]/60 font-medium">
                    {loadingMessage}
                  </div>
                </div>
                <div className="space-y-4">
                  <Skeleton className="h-4 w-3/4 bg-[#515b65]" />
                  <Skeleton className="h-4 w-full bg-[#515b65]" />
                  <Skeleton className="h-4 w-5/6 bg-[#515b65]" />
                  <Skeleton className="h-4 w-4/6 bg-[#515b65]" />
                  <Skeleton className="h-4 w-3/4 bg-[#515b65]" />
                </div>
              </div>
            ) : (
              <div className="prose prose-md3 max-w-none">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  rehypePlugins={[rehypeRaw, rehypeSanitize]}
                  components={{
                    code({ inline, className, children, ...props }: any) {
                      const match = /language-(\w+)/.exec(className || "")
                      const isInline = inline && !match

                      if (isInline) {
                        return <code className={className} {...props}>{children}</code>
                      }

                      return match ? (
                        <div className="relative group">
                          <SyntaxHighlighter
                            style={tomorrow}
                            language={match[1]}
                            PreTag="div"
                            className="!mt-0 !mb-0"
                            {...props}
                          >
                            {String(children).replace(/\n$/, "")}
                          </SyntaxHighlighter>
                          {/* Add tooltip for long file paths */}
                          {match[1] === 'filepath' && (
                            <div className="absolute inset-0 flex items-center">
                              <div className="w-full overflow-hidden text-ellipsis whitespace-nowrap px-4 py-2 text-sm text-[#f3f3f3]/70">
                                {children}
                              </div>
                              <div className="absolute inset-y-0 right-0 flex items-center">
                                <div className="h-full w-8 bg-gradient-to-l from-[#16191d] to-transparent" />
                              </div>
                            </div>
                          )}
                        </div>
                      ) : (
                        <code className={className} {...props}>
                          {children}
                        </code>
                      )
                    },
                    h1: ({ children }) => (
                      <h1 className="text-2xl font-bold mb-4 text-[#f3f3f3] border-b border-[#515b65] pb-2">
                        {children}
                      </h1>
                    ),
                    h2: ({ children }) => (
                      <h2 className="text-xl font-semibold mb-3 text-[#f3f3f3] mt-6">{children}</h2>
                    ),
                    h3: ({ children }) => <h3 className="text-lg font-medium mb-2 text-[#f3f3f3] mt-4">{children}</h3>,
                    p: ({ children }) => <p className="mb-4 leading-relaxed text-[#f3f3f3]">{children}</p>,
                    ul: ({ children }) => (
                      <ul className="list-disc list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ul>
                    ),
                    ol: ({ children }) => (
                      <ol className="list-decimal list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ol>
                    ),
                    li: ({ children }) => <li className="text-[#f3f3f3]">{children}</li>,
                    blockquote: ({ children }) => (
                      <blockquote className="border-l-4 border-[#f3c9a4] pl-4 italic my-4 text-[#f3f3f3]/80 bg-[#f3c9a4]/5 py-2 rounded-r">
                        {children}
                      </blockquote>
                    ),
                    a: ({ href, children }) => {
                      // Check if the link is a file reference (e.g., src/file.txt)
                      if (href && !href.startsWith('http')) {
                        return (
                          <FileLink
                            repoId={`${params.owner}/${params.name}`}
                            filePath={href}
                          >
                            {children}
                          </FileLink>
                        );
                      }
                      // Regular external links
                      return (
                        <a
                          href={href}
                          className="text-[#f3c9a4] hover:underline"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {children}
                        </a>
                      );
                    },
                  }}
                >
                  {guide}
                </ReactMarkdown>
              </div>
            )}
          </CardContent>
        </Card>

        {!guideLoading && <Separator className="mb-12 bg-[#515b65]" /> }

        {/* AI Chat Interface */}
        {!guideLoading && (
          <Card className="bg-[#292f36] border-[#515b65] rounded-lg shadow-lg">
            <CardHeader className="p-6">
              <CardTitle className="flex items-center gap-2 text-xl">
                <div className="relative h-6 w-6">
                  <div
                    className="absolute inset-0 h-6 w-6"
                    style={{
                      background: "linear-gradient(-45deg, #f3c9a4, #3ac8bd, #f3c9a4, #3ac8bd)",
                      backgroundSize: "400% 400%",
                      animation: "gradient-x 6s ease infinite",
                      WebkitMask: "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'%3E%3Cpath d='M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.582a.5.5 0 0 1 0 .963L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z'/%3E%3Cpath d='M20 3v4'/%3E%3Cpath d='M22 5h-4'/%3E%3Cpath d='M4 17v2'/%3E%3Cpath d='M5 18H3'/%3E%3C/svg%3E\") center/contain no-repeat",
                      mask: "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'%3E%3Cpath d='M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.582a.5.5 0 0 1 0 .963L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z'/%3E%3Cpath d='M20 3v4'/%3E%3Cpath d='M22 5h-4'/%3E%3Cpath d='M4 17v2'/%3E%3Cpath d='M5 18H3'/%3E%3C/svg%3E\") center/contain no-repeat",
                    }}
                  />
                </div>
                <span className="bg-gradient-to-r from-[#f3c9a4] to-[#3ac8bd] bg-clip-text text-transparent bg-size-200 animate-gradient-x font-semibold">
                  AI Issue Assistant
                </span>
              </CardTitle>
              <CardDescription className="text-[#f3f3f3]/60">
                Get AI-powered guidance and code examples for this issue
              </CardDescription>
            </CardHeader>
            <CardContent className="p-6">
              {/* Messages */}
              <div className="space-y-6 mb-8 custom-scrollbar relative">
                {messages.length === 0 && (
                  <div className="flex justify-start">
                    <div className="max-w-[80%] rounded-lg p-4 bg-gradient-to-r from-[#292f36] to-[#f3c9a4]/5 border border-[#515b65] shadow-md">
                      <div className="prose prose-md3 max-w-none">
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          rehypePlugins={[rehypeRaw, rehypeSanitize]}
                          components={{
                            code({ inline, className, children, ...props }: any) {
                              const match = /language-(\w+)/.exec(className || "")
                              const isInline = inline && !match // Treat as inline if explicitly inline and not a code block

                              if (isInline) {
                                // Render inline code without SyntaxHighlighter
                                return <code className={className} {...props}>{children}</code>
                              }

                              return match ? (
                                <div className="relative group">
                                  <SyntaxHighlighter
                                    style={tomorrow}
                                    language={match[1]}
                                    PreTag="div"
                                    className="!mt-0 !mb-0"
                                    {...props}
                                  >
                                    {String(children).replace(/\n$/, "")}
                                  </SyntaxHighlighter>
                                  {/* Add tooltip for long file paths */}
                                  {match[1] === 'filepath' && (
                                    <div className="absolute inset-0 flex items-center">
                                      <div className="w-full overflow-hidden text-ellipsis whitespace-nowrap px-4 py-2 text-sm text-[#f3f3f3]/70">
                                        {children}
                                      </div>
                                      <div className="absolute inset-y-0 right-0 flex items-center">
                                        <div className="h-full w-8 bg-gradient-to-l from-[#16191d] to-transparent" />
                                      </div>
                                    </div>
                                  )}
                                </div>
                              ) : (
                                <code className={className} {...props}>
                                  {children}
                                </code>
                              )
                            },
                            pre: ({ children }) => (
                              <pre className="bg-[#16191d] border border-[#515b65] rounded-lg p-4 overflow-x-auto my-4">
                                {children}
                              </pre>
                            ),
                            h1: ({ children }) => (
                              <h1 className="text-2xl font-bold mb-4 text-[#f3f3f3] border-b border-[#515b65] pb-2">
                                {children}
                              </h1>
                            ),
                            h2: ({ children }) => (
                              <h2 className="text-xl font-semibold mb-3 text-[#f3f3f3] mt-6">{children}</h2>
                            ),
                            h3: ({ children }) => <h3 className="text-lg font-medium mb-2 text-[#f3f3f3] mt-4">{children}</h3>,
                            h4: ({ children }) => (
                              <h4 className="text-base font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h4>
                            ),
                            h5: ({ children }) => <h5 className="text-sm font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h5>,
                            h6: ({ children }) => <h6 className="text-sm font-medium mb-2 text-[#f3f3f3] mt-3">{children}</h6>,
                            p: ({ children }) => <p className="mb-4 leading-relaxed text-[#f3f3f3]">{children}</p>,
                            ul: ({ children }) => (
                              <ul className="list-disc list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ul>
                            ),
                            ol: ({ children }) => (
                              <ol className="list-decimal list-inside mb-4 space-y-2 text-[#f3f3f3] ml-4">{children}</ol>
                            ),
                            li: ({ children }) => <li className="text-[#f3f3f3]">{children}</li>,
                            blockquote: ({ children }) => (
                              <blockquote className="border-l-4 border-[#f3c9a4] pl-4 italic my-4 text-[#f3f3f3]/80 bg-[#f3c9a4]/5 py-2 rounded-r">
                                {children}
                              </blockquote>
                            ),
                            a: ({ href, children }) => {
                              // Check if the link is a file reference (e.g., src/file.txt)
                              if (href && !href.startsWith('http')) {
                                return (
                                  <FileLink
                                    repoId={`${params.owner}/${params.name}`}
                                    filePath={href}
                                  >
                                    {children}
                                  </FileLink>
                                );
                              }
                              // Regular external links
                              return (
                                <a
                                  href={href}
                                  className="text-[#f3c9a4] hover:underline"
                                  target="_blank"
                                  rel="noopener noreferrer"
                                >
                                  {children}
                                </a>
                              );
                            },
                            table: ({ children }) => (
                              <div className="overflow-x-auto my-4">
                                <table className="min-w-full border border-[#515b65] rounded-lg">{children}</table>
                              </div>
                            ),
                            thead: ({ children }) => <thead className="bg-[#292f36]">{children}</thead>,
                            tbody: ({ children }) => <tbody>{children}</tbody>,
                            tr: ({ children }) => <tr className="border-b border-[#515b65]">{children}</tr>,
                            th: ({ children }) => (
                              <th className="border border-[#515b65] px-4 py-3 bg-[#292f36] font-semibold text-left text-[#f3f3f3]">
                                {children}
                              </th>
                            ),
                            td: ({ children }) => (
                              <td className="border border-[#515b65] px-4 py-3 text-[#f3f3f3]">{children}</td>
                            ),
                            strong: ({ children }) => <strong className="font-bold text-[#f3c9a4]">{children}</strong>,
                            em: ({ children }) => <em className="italic text-[#f3f3f3]/90">{children}</em>,
                            hr: () => <hr className="border-[#515b65] my-6" />,
                            img: ({ src, alt }) => (
                              <img
                                src={src || "/placeholder.svg"}
                                alt={alt || ""}
                                className="max-w-full h-auto rounded-lg border border-[#515b65] my-4"
                              />
                            ),
                          }}
                        >
                          {`# Welcome to the AI Issue Assistant

I can help you understand this issue by searching through the codebase. Here are some examples of what you can ask:

- How is this feature implemented in the code?
- Where are the relevant files for this issue?
- Can you show me the code that handles this functionality?
- What are the dependencies and imports needed for this feature?

**Try asking a question to get started!**`}
                        </ReactMarkdown>
                      </div>
                    </div>
                  </div>
                )}

                {messages.map((message) => (
                  <div
                    key={message.id}
                    className={`flex gap-4 ${message.role === "user" ? "justify-end pr-2" : "justify-start"}`}
                  >
                    <div
                      className={`max-w-[80%] rounded-lg p-4 ${
                        message.role === "user"
                          ? "bg-[#f3c9a4]/20 border border-[#f3c9a4]/30 text-[#f3f3f3] shadow-sm"
                          : "bg-gradient-to-r from-[#292f36] to-[#f3c9a4]/5 border border-[#515b65] shadow-md"
                      }`}
                    >
                      {message.role === "assistant" ? (
                        <div className="prose prose-md3 max-w-none">
                          <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            rehypePlugins={[rehypeRaw, rehypeSanitize]}
                            components={{
                              code({ inline, className, children, ...props }: any) {
                                const match = /language-(\w+)/.exec(className || "")
                                const isInline = inline && !match // Treat as inline if explicitly inline and not a code block

                                if (isInline) {
                                  // Render inline code without SyntaxHighlighter
                                  return <code className={className} {...props}>{children}</code>
                                }

                                return match ? (
                                  <div className="relative group">
                                    <SyntaxHighlighter
                                      style={tomorrow}
                                      language={match[1]}
                                      PreTag="div"
                                      className="!mt-0 !mb-0"
                                      {...props}
                                    >
                                      {String(children).replace(/\n$/, "")}
                                    </SyntaxHighlighter>
                                    {/* Add tooltip for long file paths */}
                                    {match[1] === 'filepath' && (
                                      <div className="absolute inset-0 flex items-center">
                                        <div className="w-full overflow-hidden text-ellipsis whitespace-nowrap px-4 py-2 text-sm text-[#f3f3f3]/70">
                                          {children}
                                        </div>
                                        <div className="absolute inset-y-0 right-0 flex items-center">
                                          <div className="h-full w-8 bg-gradient-to-l from-[#16191d] to-transparent" />
                                        </div>
                                      </div>
                                    )}
                                  </div>
                                ) : (
                                  <code className={className} {...props}>
                                    {children}
                                  </code>
                                )
                              },
                              h1: ({ children }) => <h1 className="text-lg font-bold mb-2 text-[#f3f3f3]">{children}</h1>,
                              h2: ({ children }) => (
                                <h2 className="text-base font-semibold mb-2 text-[#f3f3f3]">{children}</h2>
                              ),
                              h3: ({ children }) => (
                                <h3 className="text-sm font-medium mb-1 text-[#f3f3f3]">{children}</h3>
                              ),
                              p: ({ children }) => <p className="mb-2 leading-relaxed text-[#f3f3f3]">{children}</p>,
                              ul: ({ children }) => (
                                <ul className="list-disc list-inside mb-2 space-y-1 text-[#f3f3f3]">{children}</ul>
                              ),
                              ol: ({ children }) => (
                                <ol className="list-decimal list-inside mb-2 space-y-1 text-[#f3f3f3]">{children}</ol>
                              ),
                              li: ({ children }) => <li className="ml-2 text-[#f3f3f3]">{children}</li>,
                              blockquote: ({ children }) => (
                                <blockquote className="border-l-2 border-[#f3c9a4] pl-2 italic my-2 text-[#f3f3f3]/80">
                                  {children}
                                </blockquote>
                              ),
                              a: ({ href, children }) => {
                                // Check if the link is a file reference (e.g., src/file.txt)
                                if (href && !href.startsWith('http')) {
                                  return (
                                    <FileLink
                                      repoId={`${params.owner}/${params.name}`}
                                      filePath={href}
                                    >
                                      {children}
                                    </FileLink>
                                  );
                                }
                                // Regular external links
                                return (
                                  <a
                                    href={href}
                                    className="text-[#f3c9a4] hover:underline"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                  >
                                    {children}
                                  </a>
                                );
                              },
                            }}
                          >
                            {/* Render the typed portion if it exists; otherwise empty */}
                            {displayedContent[message.id] ?? ""}
                          </ReactMarkdown>
                        </div>
                      ) : (
                        <p className="leading-relaxed text-[#f3f3f3]">{message.content}</p>
                      )}
                    </div>
                  </div>
                ))}

                {isGenerating && (
                  <div className="flex gap-4 justify-start">
                    <div className="bg-[#292f36] border border-[#515b65] rounded-lg p-4 shadow-md">
                      <div className="flex items-center gap-2">
                        <div className="animate-pulse text-[#f3c9a4] font-medium">Thinking...</div>
                      </div>
                    </div>
                  </div>
                )}

                {isTyping && (
                  <div className="absolute bottom-0 right-0">
                    <Button
                      onClick={skipTyping}
                      className="bg-[#f3c9a4] hover:bg-[#d4a882] active:bg-[#c29c72] text-[#16191d] text-xs px-2 py-1 rounded-lg font-medium shadow-md hover:shadow-lg transition-all duration-200"
                    >
                      Skip Generation
                    </Button>
                  </div>
                )}

                <div ref={messagesEndRef} />
              </div>

              {/* Input Form */}
              <form onSubmit={handleSubmit} className="relative w-full">
                <Textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Ask about this issue... (e.g., 'How can I reproduce this bug?' or 'What's the best approach to fix this?')"
                  className="w-full min-h-[80px] pr-16 bg-[#16191d] border-[#515b65] rounded-lg text-[#f3f3f3] placeholder:text-[#f3c9a4]/60 focus:ring-0 focus:border-[#515b65] resize-none"
                  disabled={isGenerating}
                />
                <Button
                  type="submit"
                  disabled={!input.trim() || isGenerating}
                  className="absolute right-3 bottom-3 w-10 h-10 bg-[#f3c9a4] hover:bg-[#d4a882] active:bg-[#c29c72] text-[#16191d] rounded-full font-medium shadow-md hover:shadow-lg transition-all duration-200 flex items-center justify-center p-0"
                >
                  <ArrowUp className="h-5 w-5" />
                </Button>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
