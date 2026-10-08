import { fetchJSON, fetchURL } from "./utils";

export async function list() {
  return fetchJSON<ITrashList>(`/api/trash`, {});
}

export async function restore(ids: string[]) {
  const res = await fetchURL(`/api/trash/restore`, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
  return (await res.json()) as { id: string; path: string }[];
}

export async function purge(ids: string[]) {
  await fetchURL(`/api/trash/purge`, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

export async function empty() {
  await fetchURL(`/api/trash`, { method: "DELETE" });
}

export async function usage() {
  return fetchJSON<{ count: number; size: number }>(`/api/trash/all`, {});
}

export async function emptyAll() {
  await fetchURL(`/api/trash/all`, { method: "DELETE" });
}
