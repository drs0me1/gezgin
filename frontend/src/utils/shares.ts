import type { ShareUpdate, SharedItem } from "@/api/share";

// The "Paylaşılanlar" page's order of the shares and the changes its edit window sends (Gezgin,
// K95, K96).

const DAY = 24 * 60 * 60;

export type ShareEnd = "permanent" | "ended" | "soon" | "later";

// shareEnd tells when a share ends, at now (Unix seconds): never, already, within a day, or later.
export function shareEnd(expire: number | undefined, now: number): ShareEnd {
  if (!expire) return "permanent";
  if (expire <= now) return "ended";
  if (expire - now <= DAY) return "soon";
  return "later";
}

export interface ShareSort {
  by: "name" | "end";
  asc: boolean;
}

// sortShares orders the shares by the name shown, or by their end, the permanent ones last; equal
// ones by name.
export function sortShares(
  links: SharedItem[],
  sort: ShareSort,
  nameOf: (link: SharedItem) => string,
  collator: Intl.Collator
): SharedItem[] {
  const byName = (a: SharedItem, b: SharedItem) =>
    collator.compare(nameOf(a), nameOf(b));
  const end = (link: SharedItem) => link.expire || Number.MAX_SAFE_INTEGER;
  const compare =
    sort.by === "end"
      ? (a: SharedItem, b: SharedItem) => end(a) - end(b) || byName(a, b)
      : byName;
  const sign = sort.asc ? 1 : -1;
  return [...links].sort((a, b) => sign * compare(a, b));
}

// ShareForm is what the edit window holds: the duration kept, set from now, or ended; the password
// kept, set or removed; and a WebDAV share's write access.
export interface ShareForm {
  duration: "keep" | "set" | "permanent";
  time: number;
  unit: string;
  password: "keep" | "set" | "remove";
  newPassword: string;
  writable: boolean;
}

// shareUpdate returns the changes the form makes to the share, nothing for what it leaves as is.
export function shareUpdate(link: Share, form: ShareForm): ShareUpdate {
  const body: ShareUpdate = {};
  if (form.duration === "set") {
    body.expires = String(form.time);
    body.unit = form.unit;
  } else if (form.duration === "permanent") {
    body.expires = "0";
  }
  if (form.password === "set") {
    body.passwordAction = "set";
    body.password = form.newPassword;
  } else if (form.password === "remove") {
    body.passwordAction = "remove";
  }
  if (link.kind === "webdav" && form.writable !== !!link.writable) {
    body.writable = form.writable;
  }
  return body;
}
