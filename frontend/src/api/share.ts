import { fetchURL, fetchJSON, removePrefix, createURL } from "./utils";
import { webdavPort } from "@/utils/constants";

export async function list() {
  return fetchJSON<Share[]>("/api/shares");
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
