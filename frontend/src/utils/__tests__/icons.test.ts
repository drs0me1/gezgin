import { describe, expect, it } from "vitest";
import { outlineIcons } from "../icons";

// K106: every outline icon is drawn on Tabler's 24 by 24 grid with paths only.
describe("outlineIcons", () => {
  it("has paths for every button icon it names", () => {
    for (const [name, icon] of Object.entries(outlineIcons)) {
      expect(icon.paths.length, name).toBeGreaterThan(0);
      for (const d of icon.paths) expect(d, name).toMatch(/^M[-\d. ]/);
    }
  });

  it("draws the header's icons", () => {
    for (const name of [
      "arrow_back",
      "arrow_forward",
      "arrow_upward",
      "home",
      "search",
      "menu",
      "more_vert",
      "check_circle",
      "file_upload",
      "drive_file_move",
      "star",
      "star_border",
    ]) {
      expect(outlineIcons[name], name).toBeDefined();
    }
    // A favourite's star is filled, the other one only drawn.
    expect(outlineIcons.star.filled).toBe(true);
    expect(outlineIcons.star_border.filled).toBeUndefined();
  });
});
