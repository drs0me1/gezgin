import { fetchJSON } from "./utils";

// Favourites (Gezgin): each call answers the user's list as it is afterwards.

export async function list() {
  return fetchJSON<IFavorite[]>(`/api/favorites`, {});
}

export async function add(path: string) {
  return fetchJSON<IFavorite[]>(`/api/favorites`, {
    method: "POST",
    body: JSON.stringify({ path }),
  });
}

export async function remove(path: string) {
  return fetchJSON<IFavorite[]>(`/api/favorites`, {
    method: "DELETE",
    body: JSON.stringify({ path }),
  });
}
