import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Sovereign — Home Server Appliance",
  description: "Quiet appliance confidence for private home cloud infrastructure.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-[var(--canvas)] text-[var(--text-primary)] antialiased">
        {children}
      </body>
    </html>
  );
}
