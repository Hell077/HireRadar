import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "HireRadar",
  description: "Remote jobs matched to your experience.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
