import { describe, expect, it } from "vitest";
import type { SharedItem } from "@/api/share";
import { shareEnd, shareUpdate, sortShares, type ShareForm } from "../shares";

const now = 1_800_000_000;
const DAY = 24 * 60 * 60;

const item = (name: string, expire: number): SharedItem => ({
  hash: name,
  path: "/" + name,
  expire,
  name,
  isDir: false,
  folder: "/",
});

const keep: ShareForm = {
  duration: "keep",
  time: 7,
  unit: "days",
  password: "keep",
  newPassword: "",
  writable: false,
};

// K95: the page tells when a share ends and orders the shares by name or by end.
describe("shareEnd", () => {
  it("tells a share's end", () => {
    expect(shareEnd(0, now)).toBe("permanent");
    expect(shareEnd(undefined, now)).toBe("permanent");
    expect(shareEnd(now - 1, now)).toBe("ended");
    expect(shareEnd(now + DAY, now)).toBe("soon");
    expect(shareEnd(now + DAY + 1, now)).toBe("later");
  });
});

describe("sortShares", () => {
  const collator = new Intl.Collator("tr", { numeric: true });
  const links = [
    item("zeytin.txt", 0),
    item("ağaç", now + 5 * DAY),
    item("dosya10", now + DAY),
    item("dosya2", now + 5 * DAY),
  ];
  const names = (l: SharedItem[]) => l.map((i) => i.name);

  it("orders by name, in the language's order and numbers by value", () => {
    const byName = sortShares(
      links,
      { by: "name", asc: true },
      (l) => l.name,
      collator
    );
    expect(names(byName)).toEqual(["ağaç", "dosya2", "dosya10", "zeytin.txt"]);
    const back = sortShares(
      links,
      { by: "name", asc: false },
      (l) => l.name,
      collator
    );
    expect(names(back)).toEqual(["zeytin.txt", "dosya10", "dosya2", "ağaç"]);
  });

  it("orders by end, the permanent ones last and equal ends by name", () => {
    const byEnd = sortShares(
      links,
      { by: "end", asc: true },
      (l) => l.name,
      collator
    );
    expect(names(byEnd)).toEqual(["dosya10", "ağaç", "dosya2", "zeytin.txt"]);
    expect(names(links)).toEqual(["zeytin.txt", "ağaç", "dosya10", "dosya2"]);
  });
});

// K96: the edit window sends only what it changes.
describe("shareUpdate", () => {
  const link: Share = { hash: "h", path: "/a", expire: now, hasPassword: true };
  const dav: Share = {
    ...link,
    kind: "webdav",
    webdavUser: "film",
    writable: false,
  };

  it("sends nothing for a form left as it is", () => {
    expect(shareUpdate(link, keep)).toEqual({});
    expect(shareUpdate(dav, keep)).toEqual({});
  });

  it("sends a new duration, or none", () => {
    expect(
      shareUpdate(link, { ...keep, duration: "set", time: 3, unit: "hours" })
    ).toEqual({
      expires: "3",
      unit: "hours",
    });
    expect(shareUpdate(link, { ...keep, duration: "permanent" })).toEqual({
      expires: "0",
    });
  });

  it("sends a password set or removed", () => {
    expect(
      shareUpdate(link, { ...keep, password: "set", newPassword: "yeni" })
    ).toEqual({
      passwordAction: "set",
      password: "yeni",
    });
    expect(
      shareUpdate(link, { ...keep, password: "remove", newPassword: "x" })
    ).toEqual({
      passwordAction: "remove",
    });
  });

  it("sends a WebDAV share's write access only when it changes, and never a link's", () => {
    expect(shareUpdate(dav, { ...keep, writable: true })).toEqual({
      writable: true,
    });
    expect(
      shareUpdate({ ...dav, writable: true }, { ...keep, writable: false })
    ).toEqual({
      writable: false,
    });
    expect(shareUpdate(link, { ...keep, writable: true })).toEqual({});
  });
});
