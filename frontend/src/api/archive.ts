import { fetchURL, fetchJSON, StatusError } from "./utils";
import i18n from "@/i18n";

// Gezgin opens and makes archives on the server, in jobs that run one at a time.

export interface ArchiveJob {
  id: string;
  kind: "extract" | "create";
  folder: string;
  names: string[];
  state: "running" | "done" | "failed" | "cancelled";
  error?: string;
  // The folder an extraction opened into, or the archive made (its first volume for a set).
  result?: string;
  bytes: number;
  entries: number;
  archives: number;
  // A creation's archive name, format and progress.
  name?: string;
  format?: string;
  planned?: boolean;
  files?: number;
  total?: number;
  skipped?: number;
  windows?: number;
  volumes?: number;
  started: string;
  finished?: string;
}

export async function list() {
  return fetchJSON<ArchiveJob[]>(`/api/archive`);
}

async function start(body: object) {
  try {
    const res = await fetchURL(`/api/archive`, {
      method: "POST",
      body: JSON.stringify(body),
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

// extract opens the archives at paths, all in one folder, with the password when one is given.
export async function extract(paths: string[], password: string) {
  return start({ items: paths, password });
}

// create packs the items at paths, all in one folder, into an archive named name beside them,
// in volumes of volume bytes when that is not 0. ZIP's times are written in the browser's zone,
// which Windows shows them in.
export async function create(
  paths: string[],
  name: string,
  format: string,
  volume: number
) {
  return start({
    kind: "create",
    items: paths,
    name,
    format,
    volume,
    zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  });
}

export async function cancel(id: string) {
  await fetchURL(`/api/archive/${id}`, { method: "DELETE" });
}
