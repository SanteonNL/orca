import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";
import { Toaster } from "@/components/ui/sonner";
import { QueryProvider } from "@/components/query-provider";

const fsMeFont = localFont({
  src: [
    { path: "./public/fonts/fsme/FSMe.otf", weight: "400", style: "normal" },
    { path: "./public/fonts/fsme/FSMe-Bold.otf", weight: "700", style: "normal" },
    { path: "./public/fonts/fsme/FSMe-Italic.otf", weight: "400", style: "italic" },
    { path: "./public/fonts/fsme/FSMe-BoldItalic.otf", weight: "700", style: "italic" },
    { path: "./public/fonts/fsme/FSMe-Light.otf", weight: "300", style: "normal" },
    { path: "./public/fonts/fsme/FSMe-LightItalic.otf", weight: "300", style: "italic" },
    { path: "./public/fonts/fsme/FSMe-Heavy.otf", weight: "900", style: "normal" },
    { path: "./public/fonts/fsme/FSMe-HeavyItalic.otf", weight: "900", style: "italic" }
  ],
  variable: "--font-fsme",
  display: "swap"
});

export const metadata: Metadata = {
  title: process.env.NEXT_PUBLIC_TITLE || "ORCA Frontend",
  description: "This app renders Questionnaires based on the SDC specification. It allows users to input required data before a Task is published to placer(s)",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={fsMeFont.className}>
        <QueryProvider>
          <main>
            {children}
            <Toaster />
          </main>
        </QueryProvider>
      </body>
    </html>
  );
}
