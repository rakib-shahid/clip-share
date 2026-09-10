import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
export default defineConfig({
    plugins: [react(), tailwindcss()],
    resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
    server: {
        port: 5173,
        proxy: {
            "/api": "http://api:8080",
            "/healthz": "http://api:8080",
            "/readyz": "http://api:8080",
            "/c": "http://api:8080",
            "/m": "http://api:8080"
        }
    }
});
