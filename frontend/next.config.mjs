/** @type {import('next').NextConfig} */
const nextConfig = {
    // output: "standalone",
    basePath: process.env.NEXT_PUBLIC_BASE_PATH || "",
    images: {
        remotePatterns: [
            {
                protocol: 'https',
                hostname: 'randomuser.me',
                port: '',
                pathname: '/api/portraits/**',
            },
        ]
    }
};

export default nextConfig;
