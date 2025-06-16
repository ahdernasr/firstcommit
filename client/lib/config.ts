// API URL configuration
export const config = {
  apiUrl: process.env.NEXT_PUBLIC_API_URL || 
    (process.env.NODE_ENV === 'development' 
      ? 'http://localhost:8080' 
      : 'https://backend-222198140851.us-central1.run.app'),
  env: process.env.NODE_ENV || 'development'
} as const;

// Helper function to get API URL
export const getApiUrl = () => {
  console.log('Environment:', config.env)
  console.log('NEXT_PUBLIC_API_URL:', process.env.NEXT_PUBLIC_API_URL)
  console.log('Using API URL:', config.apiUrl)
  return config.apiUrl
}

// Helper function to get full API endpoint URL
export const getApiEndpoint = (endpoint: string) => {
  const url = `${config.apiUrl}${endpoint.startsWith('/') ? endpoint : `/${endpoint}`}`
  console.log('Making request to:', url)
  return url
} 