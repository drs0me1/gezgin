import { fetchURL, fetchJSON, StatusError } from "./utils";
import i18n from "@/i18n";

// Gezgin opens archives on the server, in jobs that run one at a time.

export interface ArchiveJob {
  id: string;
  folder: string;
  names: string[];
  state: "running" | "done" | "failed" | "cancelled";
  error?: string;
  result?: string;
  bytes: number;
  entries: number;
  archives: number;
  started: string;
  finished?: string;
}

export async function list() {
  return fetchJSON<ArchiveJob[]>(`/api/archive`);
}

// extract opens the archives at paths, all in one folder, with the password when one is given.
export async function extract(paths: string[], password: string) {
  try {
    const res = await fetchURL(`/api/archive`, {
      method: "POST",
      body: JSON.stringify({ items: paths, password }),
    });
    return (await res.json()) as ArchiveJob;
  } catch (e) {
    if (e instanceof StatusError && e.status === 409) {
      throw new Error(i18n.global.t("archive.busy"));
    }
    if (e instanceof StatusError && e.status === 507) {
      throw new Error(i18n.global.t("errors.noSpace"));
    }
    throw e;
  }
}

export async function cancel(id: string) {
  await fetchURL(`/api/archive/${id}`, { method: "DELETE" });
}
