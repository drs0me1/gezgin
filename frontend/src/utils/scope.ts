// A user's access (Gezgin, K148): all files, their own folder in the folder of the users'
// folders, or a folder chosen in the tree.
export type ScopeMode = "all" | "own" | "folder";

// cleanUsername names a user's own folder as the server does (settings.cleanUsername).
export const cleanUsername = (name: string) =>
  name
    .replace(/^ +| +$/g, "")
    .split("..")
    .join("")
    .replace(/[^0-9A-Za-z@_\-.]/g, "-")
    .replace(/-+/g, "-");

// homeDir is a user's own folder, as the server makes it.
export const homeDir = (base: string, username: string) =>
  ("/" + base + "/" + cleanUsername(username)).replace(/\/+/g, "/");

// isAll tells whether a scope is every file.
export const isAll = (scope: string) => ["", ".", "/"].includes(scope.trim());

// scopeMode tells how a scope reads in the access choice.
export const scopeMode = (
  scope: string,
  own: string | null
): { mode: ScopeMode; folder: string } => {
  if (isAll(scope)) return { mode: "all", folder: "" };
  const clean = ("/" + scope).replace(/\/+/g, "/").replace(/\/$/, "");
  if (own !== null && clean === own) return { mode: "own", folder: "" };
  return { mode: "folder", folder: clean };
};

// folderOfURL turns a folder's address in the file list ("/files/a%20b/") into its path in the
// files ("/a b"), under the scope of the admin who picked it.
export const folderOfURL = (url: string, base: string) => {
  const inside = url
    .replace(/^\/files/, "")
    .split("/")
    .filter((part) => part !== "")
    .map(decodeURIComponent)
    .join("/");
  const root = isAll(base) ? "" : ("/" + base).replace(/\/+$/, "");
  return (root + "/" + inside).replace(/\/+/g, "/").replace(/(.)\/$/, "$1");
};
