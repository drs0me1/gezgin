import { fetchJSON } from "./utils";

// Favourites (Gezgin): each call answers the user's favourites as they are afterwards, as a
// listing; sizes asks for the folders' item counts and sizes, which only the page shows.

export async function list(sizes = false) {
  return fetchJSON<IFavoritesListing>(
    `/api/favorites${sizes ? "?sizes=true" : ""}`,
    {}
  );
}

export async function add(path: string) {
  return fetchJSON<IFavoritesListing>(`/api/favorites`, {
    method: "POST",
    body: JSON.stringify({ path }),
  });
}

export async function remove(path: string) {
  return fetchJSON<IFavoritesListing>(`/api/favorites`, {
    method: "DELETE",
    body: JSON.stringify({ path }),
  });
}
