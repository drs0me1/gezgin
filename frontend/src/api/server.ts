import { fetchJSON, fetchURL } from "./utils";

// What the container runs with and the sizes Gezgin keeps beside the files (Gezgin, K146, K147).
export interface ServerInfo {
  version: string;
  webdavPort: string;
  thumbnails: boolean;
  sessionSeconds: number;
  databaseSize: number;
  cache: { count: number; size: number } | null;
}

export function get() {
  return fetchJSON<ServerInfo>(`/api/server`, {});
}

export async function clearCache() {
  await fetchURL(`/api/server/cache`, { method: "DELETE" });
}
