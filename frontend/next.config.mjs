/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: "standalone",
  async redirects() {
    return [
      {
        source: "/",
        destination: "/tasks/protein-binder-design",
        permanent: true,
      },
    ];
  },
};

export default nextConfig;
