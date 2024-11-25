import type { Metadata } from "next";
import { Nunito } from "next/font/google";
import "./globals.css";
import { Toaster } from "@/components/ui/sonner";

const nunito = Nunito({ subsets: ["latin"] });

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
      <body className={nunito.className}>
        <main className="h-screen w-screen">
          {children}
          <Toaster />
        </main>
      </body>
    </html>
  );
}
