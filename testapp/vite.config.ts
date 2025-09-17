/// <reference types="vitest/config" />
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [],
  test: {
    globalSetup: ["./tests/setup.ts"],
    teardownTimeout: 1000,
  },
});
