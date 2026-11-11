/// <reference types="vitest/config" />

import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
    plugins: [vue(), vueDevTools()],
    // Keep the lazy CAD preview's Three.js dependencies in separate chunks.
    // Revalidate these module paths when upgrading Three.js.
    build: {
        rolldownOptions: {
            output: {
                codeSplitting: {
                    groups: [
                        {
                            name: 'three-core',
                            test: /node_modules[\\/]three[\\/]build[\\/]three\.core\.js$/,
                        },
                        {
                            name: 'three-main',
                            test: /node_modules[\\/]three[\\/]build[\\/]three\.module\.js$/,
                        },
                        {
                            name: 'three-addons',
                            test: /node_modules[\\/]three[\\/]examples[\\/]/,
                        },
                    ],
                },
            },
        },
    },
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url)),
        },
    },
    server: {
        proxy: {
            '/api': {
                target: 'http://api:8080',
                changeOrigin: false,
            },
        },
    },
    test: {
        environment: 'jsdom',
    },
})
