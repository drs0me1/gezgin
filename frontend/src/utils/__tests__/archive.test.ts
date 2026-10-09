import { describe, expect, it } from "vitest";
import {
  defaultArchiveName,
  isArchive,
  volumeBytes,
  volumeSizes,
} from "../archive";

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
      "film.zip.001",
      "Film.ZIP.002",
      "kod.tar.gz.003",
      "belge.7z.001",
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
      "film.mkv.001",
      "x.001",
      "x.rar.001",
      "x.zip.01",
      "zip",
      "rar",
    ]) {
      expect(isArchive(name), name).toBe(false);
    }
  });
});

describe("making archives", () => {
  it("names an archive after what it holds", () => {
    expect(defaultArchiveName([{ name: "Tatil", isDir: true }], "medya")).toBe(
      "Tatil"
    );
    expect(
      defaultArchiveName([{ name: "film.v2.mkv", isDir: false }], "medya")
    ).toBe("film.v2");
    expect(defaultArchiveName([{ name: ".bashrc", isDir: false }], "")).toBe(
      ".bashrc"
    );
    expect(
      defaultArchiveName(
        [
          { name: "a.txt", isDir: false },
          { name: "b", isDir: true },
        ],
        "medya"
      )
    ).toBe("medya");
    expect(defaultArchiveName([], "")).toBe("arsiv");
  });

  it("splits in volumes a FAT32 drive takes", () => {
    expect(volumeSizes.fat32).toBeLessThan(4 * 1024 ** 3);
    expect(volumeBytes(25)).toBe(25 * 1024 * 1024);
    expect(volumeBytes(0)).toBe(0);
    expect(volumeBytes(1.5)).toBe(0);
    expect(volumeBytes(NaN)).toBe(0);
  });
});
