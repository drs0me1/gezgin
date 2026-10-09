// Back, forward, up and home in the header (Gezgin, K99-K103).

const FILES = "/files";

// parentOf returns the folder that the folder at a route path lies in ("/files/a/" for
// "/files/a/b/"), or null at the top of the user's files and off them.
export function parentOf(path: string): string | null {
  if (path !== FILES && !path.startsWith(FILES + "/")) return null;
  const rest = path.slice(FILES.length).replace(/\/+$/, "");
  if (rest === "") return null;
  return FILES + rest.slice(0, rest.lastIndexOf("/") + 1);
}

// atHome tells whether a route path is the top of the user's files.
export function atHome(path: string): boolean {
  return path === FILES || path === FILES + "/";
}

// isMac tells whether the keys are a Mac's: ⌘ in place of Alt.
export function isMac(platform: string): boolean {
  return /Mac|iPhone|iPad|iPod/.test(platform);
}

export interface NavKeys {
  back: string;
  forward: string;
  up: string;
}

// navKeys names the keys that go back, forward and up, as the browser and the system have them.
export function navKeys(mac: boolean): NavKeys {
  return mac
    ? { back: "⌘[", forward: "⌘]", up: "⌘↑" }
    : { back: "Alt+←", forward: "Alt+→", up: "Alt+↑" };
}

// isUpKey tells whether a key press asks for the parent folder: Alt+↑, or ⌘↑ on a Mac, as in
// Explorer and Finder.
export function isUpKey(
  event: Pick<
    KeyboardEvent,
    "key" | "altKey" | "metaKey" | "ctrlKey" | "shiftKey"
  >,
  mac: boolean
): boolean {
  if (event.key !== "ArrowUp" || event.shiftKey || event.ctrlKey) return false;
  return mac ? event.metaKey && !event.altKey : event.altKey && !event.metaKey;
}
