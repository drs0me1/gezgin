import { describe, expect, it } from "vitest";
import { atHome, isMac, isUpKey, navKeys, parentOf } from "../navigation";

// K101-K102: up opens the folder the open one lies in; home is the top of the user's files.
describe("parentOf", () => {
  it("gives the folder a folder lies in", () => {
    expect(parentOf("/files/a/b/")).toBe("/files/a/");
    expect(parentOf("/files/a/b")).toBe("/files/a/");
    expect(parentOf("/files/a/")).toBe("/files/");
    expect(parentOf("/files/alb%C3%BCm/2024/")).toBe("/files/alb%C3%BCm/");
  });

  it("has none at the top or off the user's files", () => {
    expect(parentOf("/files/")).toBeNull();
    expect(parentOf("/files")).toBeNull();
    expect(parentOf("/favorites")).toBeNull();
    expect(parentOf("/filesx/a/")).toBeNull();
  });
});

describe("atHome", () => {
  it("is the top of the user's files only", () => {
    expect(atHome("/files/")).toBe(true);
    expect(atHome("/files")).toBe(true);
    expect(atHome("/files/a/")).toBe(false);
    expect(atHome("/trash")).toBe(false);
  });
});

// K100-K101: the keys named and taken are the system's: Alt on Windows and Linux, ⌘ on a Mac.
describe("keys", () => {
  it("tells a Mac", () => {
    expect(isMac("MacIntel")).toBe(true);
    expect(isMac("iPhone")).toBe(true);
    expect(isMac("Win32")).toBe(false);
    expect(isMac("Linux x86_64")).toBe(false);
  });

  it("names the keys", () => {
    expect(navKeys(false)).toEqual({
      back: "Alt+←",
      forward: "Alt+→",
      up: "Alt+↑",
    });
    expect(navKeys(true)).toEqual({ back: "⌘[", forward: "⌘]", up: "⌘↑" });
  });

  const key = (k: Partial<KeyboardEvent>) => ({
    key: "ArrowUp",
    altKey: false,
    metaKey: false,
    ctrlKey: false,
    shiftKey: false,
    ...k,
  });

  it("takes Alt+↑, or ⌘↑ on a Mac, and nothing else", () => {
    expect(isUpKey(key({ altKey: true }), false)).toBe(true);
    expect(isUpKey(key({ metaKey: true }), true)).toBe(true);
    expect(isUpKey(key({}), false)).toBe(false);
    expect(isUpKey(key({ metaKey: true }), false)).toBe(false);
    expect(isUpKey(key({ altKey: true }), true)).toBe(false);
    expect(isUpKey(key({ altKey: true, shiftKey: true }), false)).toBe(false);
    expect(isUpKey(key({ altKey: true, ctrlKey: true }), false)).toBe(false);
    expect(isUpKey(key({ key: "ArrowDown", altKey: true }), false)).toBe(false);
  });
});
