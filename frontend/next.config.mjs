/** @type {import('next').NextConfig} */
let allowedOrigins = [];
if (process.env.NEXT_ALLOWED_ORIGINS) {
    allowedOrigins = process.env.NEXT_ALLOWED_ORIGINS.split(',');
}
const nextConfig = {
    output: "standalone",
    basePath: process.env.NEXT_PUBLIC_BASE_PATH || "",
    experimental: {
        serverActions: {
            allowedOrigins: allowedOrigins,
        },
    },
};

export default nextConfig;

