import { describe, expect, it } from "vitest";
import { isArchive } from "../archive";

// The names have to match the server's (unpack.IsArchive).
describe("isArchive", () => {
  it("knows archives and the parts of RAR sets", () => {
    for (const name of [
      "film.zip",
      "Film.ZIP",
      "x.7z",
      "set.part01.rar",
      "rg-42386.rar",
      "rg-42386.r15",
      "set.part1of3.rar",
      "kod.tar",
      "kod.tar.gz",
      "kod.tgz",
      "kod.tar.bz2",
      "kod.tbz2",
      "kod.tbz",
      "kod.tar.xz",
      "kod.txz",
      "yedek.tar.zst",
      "yedek.tzst",
    ]) {
      expect(isArchive(name), name).toBe(true);
    }
  });

  it("leaves other files alone", () => {
    for (const name of [
      "film.mkv",
      "notlar.txt",
      "x.gz",
      "rapor.pdf",
      "x.r1",
      "a.s01",
      "fw.s19",
      "oyun.z64",
      "x.z01",
      "zip",
      "rar",
    ]) {
      expect(isArchive(name), name).toBe(false);
    }
  });
});
