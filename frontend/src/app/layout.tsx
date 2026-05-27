import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { Nav } from "@/components/nav";
import { ClientProviders } from "@/components/providers/client-providers";
import { ThemeScript } from "@/components/ui/theme-script";
import { BackgroundCanvas } from "@/components/ui/background-canvas";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Meltica Control - Trading Strategy Management",
  description: "Control plane for Meltica trading gateway",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <ThemeScript />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} relative min-h-screen bg-background antialiased`}
      >
        <ClientProviders>
          <BackgroundCanvas interactive className="fixed inset-0 -z-10 rounded-none" />
          <div className="relative z-0 flex min-h-screen flex-col bg-background/40 dark:bg-background/80 backdrop-blur-sm">
            <Nav />
            <main className="relative z-10 w-full px-4 py-6 md:px-6">
              <div className="mx-auto w-full max-w-6xl">
                {children}
              </div>
            </main>
          </div>
        </ClientProviders>
      </body>
    </html>
  );
}
