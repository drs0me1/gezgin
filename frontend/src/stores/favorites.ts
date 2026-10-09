import { defineStore } from "pinia";
import { favorites as api } from "@/api";

// The user's favourites (Gezgin), read from the server once and kept as each change answers them.
export const useFavoritesStore = defineStore("favorites", {
  state: (): { items: IFavorite[]; loaded: boolean } => ({
    items: [],
    loaded: false,
  }),
  getters: {
    has: (state) => (path: string) =>
      state.items.some((f) => f.path === trimSlash(path)),
  },
  actions: {
    async load() {
      this.items = await api.list();
      this.loaded = true;
    },
    async add(path: string) {
      this.items = await api.add(trimSlash(path));
    },
    async remove(path: string) {
      this.items = await api.remove(trimSlash(path));
    },
  },
});

// The server names a folder without its trailing slash.
function trimSlash(path: string) {
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}
