import { defineConfig } from "vite-plus";

export default defineConfig({
  lint: { options: { typeAware: true, typeCheck: true } },
  test: { include: ["src/**/*.test.ts"], maxWorkers: 2 },
  build: {
    target: "es2022",
    outDir: "../internal/serverui/dist",
    emptyOutDir: true,
    minify: false,
    lib: {
      entry: {
        controller: "src/controller.ts",
        launcher: "src/launcher.ts",
        status: "src/status.ts",
        settings: "src/settings.ts",
      },
      formats: ["es"],
      fileName: (_format, name) => `${name}.js`,
    },
  },
});
