import { defineStore } from "pinia";
import { favorites as api } from "@/api";

// The paths of the user's favourites (Gezgin), read from the server once and kept as each change
// answers them; the star asks it whether an item is one.
export const useFavoritesStore = defineStore("favorites", {
  state: (): { paths: string[]; loaded: boolean } => ({
    paths: [],
    loaded: false,
  }),
  getters: {
    has: (state) => (path: string) => state.paths.includes(trimSlash(path)),
  },
  actions: {
    keep(listing: IFavoritesListing) {
      this.paths = listing.items.map((item) => item.path);
      this.loaded = true;
    },
    async load() {
      this.keep(await api.list());
    },
    async add(path: string) {
      this.keep(await api.add(trimSlash(path)));
    },
    async remove(path: string) {
      this.keep(await api.remove(trimSlash(path)));
    },
  },
});

// The server names a folder without its trailing slash.
function trimSlash(path: string) {
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}
