import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import Navbar from "../components/navbar";
import { Toaster } from "@/components/ui/sonner";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "ORCA Frontend",
  description: "This app renders Questionnaires based on the SDC specification. It allows users to input required data before a Task is published to placer(s)",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Navbar />
        <main className="p-4 md:p-10 h-full mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          {children}
          <Toaster />
        </main>
      </body>
    </html>
  );
}
