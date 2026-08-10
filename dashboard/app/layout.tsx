import type { Metadata } from "next";
import { headers } from "next/headers";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });
export async function generateMetadata(): Promise<Metadata> { const requestHeaders = await headers(); const host = requestHeaders.get("x-forwarded-host") || requestHeaders.get("host") || "localhost:3000"; const protocol = requestHeaders.get("x-forwarded-proto") || "http"; return { metadataBase: new URL(`${protocol}://${host}`), title: "MiniPaaS — Control Plane", description: "Deploy, logs e operação para a sua MiniPaaS.", openGraph: { title: "MiniPaaS", description: "Deploy. Observe. Control.", images: ["/og.png"] }, twitter: { card: "summary_large_image", title: "MiniPaaS", description: "Deploy. Observe. Control.", images: ["/og.png"] } }; }
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="pt-BR" suppressHydrationWarning><body className={`${geistSans.variable} ${geistMono.variable}`}>{children}</body></html>; }
