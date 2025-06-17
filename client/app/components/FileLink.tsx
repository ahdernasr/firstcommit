'use client';

import { useRouter } from 'next/navigation';

interface FileLinkProps {
  repoId: string;
  filePath: string;
  children: React.ReactNode;
}

export default function FileLink({ repoId, filePath, children }: FileLinkProps) {
  const router = useRouter();

  const handleClick = (e: React.MouseEvent) => {
    e.preventDefault();
    // Remove any repo ID prefix from the file path if it exists
    const cleanFilePath = filePath.startsWith(repoId + '/') 
      ? filePath.slice(repoId.length + 1) 
      : filePath;
    
    console.log('FileLink - Original path:', filePath);
    console.log('FileLink - Cleaned path:', cleanFilePath);
    console.log('FileLink - Repo ID:', repoId);
    
    // Open in new tab
    window.open(`/repo/${repoId}/file?path=${encodeURIComponent(cleanFilePath)}`, '_blank');
  };

  return (
    <a
      href="#"
      onClick={handleClick}
      className="text-[#f3c9a4] hover:underline cursor-pointer"
    >
      {children}
    </a>
  );
} 