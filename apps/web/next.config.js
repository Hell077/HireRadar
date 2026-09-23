/** @type {import('next').NextConfig} */
import process from "node:process";

const backendUrl = process.env.BACKEND_URL ?? "http://localhost:8080";

const nextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${backendUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
