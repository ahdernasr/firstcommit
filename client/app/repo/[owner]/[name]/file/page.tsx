'use client';

import { useEffect, useState } from 'react';
import { useSearchParams, useParams } from 'next/navigation';
import { getApiEndpoint } from '@/lib/config';
import SyntaxHighlighter from "react-syntax-highlighter/dist/esm/prism";
import { tomorrow } from "react-syntax-highlighter/dist/esm/styles/prism";

interface FileContent {
  content: string;
  repo_id: string;
  file: string;
}

export default function FileViewer() {
  const searchParams = useSearchParams();
  const params = useParams();
  const [fileContent, setFileContent] = useState<FileContent | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const filePath = searchParams.get('path');
    const repoId = params.owner && params.name ? `${params.owner}/${params.name}` : null;

    if (!repoId || !filePath) {
      setError('Missing repository ID or file path');
      setLoading(false);
      return;
    }

    const fetchFileContent = async () => {
      try {
        const fullUrl = getApiEndpoint(`/api/v1/file/${repoId}/${filePath}`);
        console.log('Fetching file content from:', fullUrl);
        
        const response = await fetch(fullUrl, {
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json'
          }
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error('File API error:', {
            status: response.status,
            statusText: response.statusText,
            error: errorText,
            url: fullUrl
          });
          throw new Error(`Failed to fetch file content: ${errorText}`);
        }

        const data = await response.json();
        setFileContent(data);
      } catch (err) {
        console.error('Error fetching file content:', err);
        setError(err instanceof Error ? err.message : 'An error occurred');
      } finally {
        setLoading(false);
      }
    };

    fetchFileContent();
  }, [searchParams, params]);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-32 w-32 border-t-2 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-red-500 text-xl">{error}</div>
      </div>
    );
  }

  if (!fileContent) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-500 text-xl">No file content available</div>
      </div>
    );
  }

  // Determine file extension for syntax highlighting
  const fileExtension = fileContent.file.split('.').pop() || '';

  return (
    <div className="min-h-screen bg-[#16191d] p-8">
      <div className="max-w-4xl mx-auto">
        <div className="mb-4 text-[#f3f3f3]">
          <h1 className="text-2xl font-bold mb-2">{fileContent.file}</h1>
          <p className="text-[#f3f3f3]/70">Repository: {params.owner}/{params.name}</p>
        </div>
        <div className="bg-[#292f36] rounded-lg overflow-hidden">
          <SyntaxHighlighter
            language={fileExtension}
            style={tomorrow}
            customStyle={{
              margin: 0,
              padding: '1rem',
              background: '#292f36',
              fontSize: '0.875rem',
              lineHeight: '1.5',
            }}
          >
            {fileContent.content}
          </SyntaxHighlighter>
        </div>
      </div>
    </div>
  );
} 