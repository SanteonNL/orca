import type { Metadata } from "next";
import { Roboto } from "next/font/google";
import "./globals.css";
import { Toaster } from "@/components/ui/sonner";

const font = Roboto({
  subsets: ["latin"],
  weight: "400"
});

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
      <body className={font.className}>
        <main className="h-screen w-screen">
          {children}
          <Toaster />
        </main>
      </body>
    </html>
  );
}
