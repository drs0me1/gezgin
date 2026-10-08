import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";
import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";

// aceAssets ships the editor's modes, themes, workers and snippets with Gezgin instead of loading
// them from a CDN. They are emitted gzipped only: the server answers a .js request under /static
// from its .gz, so the plain files would only take space.
function aceAssets(): Plugin {
  const root = path.resolve(
    __dirname,
    "node_modules/ace-builds/src-min-noconflict"
  );
  return {
    name: "gezgin-ace-assets",
    apply: "build",
    generateBundle() {
      const walk = (dir: string) => {
        for (const entry of fs.readdirSync(path.join(root, dir), {
          withFileTypes: true,
        })) {
          const rel = path.posix.join(dir, entry.name);
          if (entry.isDirectory()) {
            walk(rel);
          } else if (entry.name.endsWith(".js") && rel !== "ace.js") {
            this.emitFile({
              type: "asset",
              fileName: `ace/${rel}.gz`,
              source: zlib.gzipSync(fs.readFileSync(path.join(root, rel)), {
                level: 9,
              }),
            });
          }
        }
      };
      walk("");
    },
  };
}

const plugins = [
  aceAssets(),
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  compression({ include: /\.js$/, deleteOriginalAssets: false }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(__dirname, "src")}/`,
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: "ws://127.0.0.1:8080",
            ws: true,
          },
          "/api": "http://127.0.0.1:8080",
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(__dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
