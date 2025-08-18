import type { Metadata } from "next";
import { Roboto } from "next/font/google";
import "./globals.css";
import { Toaster } from "@/components/ui/sonner";
import { QueryProvider } from "@/components/query-provider";

const font = Roboto({
  subsets: ["latin"],
  weight: ["300", "400", "500", "700"]

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
      <body className={font.className}>
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
