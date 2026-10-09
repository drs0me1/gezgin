import { fetchURL, fetchJSON, removePrefix, createURL } from "./utils";
import { webdavPort } from "@/utils/constants";

// SharedItem is a share as the "Paylaşılanlar" page lists it (Gezgin, K95): its item's name and
// kind, the folder it lies in and, when the user can reach it, where it opens; and for an admin,
// whose share it is when not their own.
export interface SharedItem extends Share {
  name: string;
  isDir: boolean;
  type?: ResourceType;
  folder: string;
  open?: string;
  owner?: string;
}

export async function list() {
  return fetchJSON<SharedItem[]>("/api/shares");
}

export async function get(url: string) {
  url = removePrefix(url);
  return fetchJSON<Share>(`/api/share${url}`);
}

export async function remove(hash: string) {
  await fetchURL(`/api/share/${hash}`, {
    method: "DELETE",
  });
}

// ShareUpdate changes a share where it is, its address kept (Gezgin, K96): a duration counted from
// now ("0" for none), a password set or removed, a WebDAV share made read-only or read-write.
export interface ShareUpdate {
  expires?: string;
  unit?: string;
  passwordAction?: "keep" | "set" | "remove";
  password?: string;
  writable?: boolean;
}

export async function update(hash: string, body: ShareUpdate) {
  return fetchJSON<Share>(`/api/share/${hash}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

// WebDAVShare names the user and whether it may write, for a share served over WebDAV (Gezgin).
export interface WebDAVShare {
  kind: "webdav";
  webdavUser: string;
  writable: boolean;
}

export async function create(
  url: string,
  password = "",
  expires = "",
  unit = "hours",
  webdav?: WebDAVShare
) {
  url = removePrefix(url);
  url = `/api/share${url}`;
  if (expires !== "") {
    url += `?expires=${expires}&unit=${unit}`;
  }
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours" || webdav) {
    body = JSON.stringify({
      password: password,
      expires: expires.toString(), // backend expects string not number
      unit: unit,
      ...webdav,
    });
  }
  return fetchJSON(url, {
    method: "POST",
    body: body,
  });
}

// webdavURL is the address a WebDAV client connects to: the host Gezgin is reached by, on the
// WebDAV port.
export function webdavURL(hash: string) {
  return `http://${window.location.hostname}:${webdavPort}/${hash}/`;
}

export function getShareURL(share: Share) {
  if (share.kind === "webdav") {
    return webdavURL(share.hash);
  }
  return createURL("share/" + share.hash, {});
}
