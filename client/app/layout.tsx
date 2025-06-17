import type React from "react"
import type { Metadata } from "next"
import "./globals.css"
import { Inter, Roboto_Mono, Oswald } from "next/font/google" // Add Oswald

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-inter",
})

const roboto_mono = Roboto_Mono({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-roboto-mono",
})

const oswald = Oswald({
  // Define Oswald font
  subsets: ["latin"],
  display: "swap",
  variable: "--font-oswald",
})

export const metadata: Metadata = {
  title: "FirstCommit",
  description: "Created with v0",
  generator: "v0.dev",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${roboto_mono.variable} ${oswald.variable} antialiased`} // Add oswald.variable
    >
      <body>{children}</body>
    </html>
  )
}
