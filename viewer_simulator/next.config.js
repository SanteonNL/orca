/** @type {import('next').NextConfig} */
let allowedOrigins = [];
if (process.env.NEXT_ALLOWED_ORIGINS) {
  allowedOrigins = process.env.NEXT_ALLOWED_ORIGINS.split(',');
}
const nextConfig = {
  output: "standalone",
  reactStrictMode: true,
  basePath: process.env.NEXT_PUBLIC_BASE_PATH || "",
  experimental: {
    serverActions: {
      allowedOrigins: allowedOrigins,
    },
  },
};

if (process.env.NODE_ENV === "development") {
  // Omit NextJS writing healthchecks to its log https://github.com/vercel/next.js/discussions/65992
  const __write = process.stdout.write;
  // @ts-ignore
  process.stdout.write = (...args) => {
    if (
        typeof args[0] !== "string" ||
        !(
            args[0].startsWith(" GET /") ||
            args[0].startsWith(" POST /") ||
            args[0].startsWith(" DELETE /") ||
            args[0].startsWith(" PATCH /")
        )
    ) {
      // @ts-ignore
      __write.apply(process.stdout, args);
    }
  };
}

module.exports = nextConfig;
