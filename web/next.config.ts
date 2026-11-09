import type { NextConfig } from "next";

// In dev, next serves the pages on :3000 and passes /api (and the frame
// previews under /dev) through to the go
// server on :8080, so cookies and passkeys see one origin. In production the
// go server serves both and this export is just files.
const api = process.env.API_ORIGIN ?? "http://localhost:8080";

const config: NextConfig = {
    output: "export",
    reactStrictMode: true,
    trailingSlash: false,
    images: { unoptimized: true },
    turbopack: { root: __dirname },
    ...(process.env.NODE_ENV === "development"
        ? {
              rewrites: async () => [
                  { source: "/api/:path*", destination: `${api}/api/:path*` },
                  { source: "/dev/:path*", destination: `${api}/dev/:path*` },
              ],
          }
        : {}),
};

export default config;
