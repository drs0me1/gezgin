import { describe, expect, it } from "vitest";
import { folderLine, knownSize, totalSize } from "../folder";

const items = (n: number) => `${n} öğe`;
const dir = (count?: number, size = 4096, sizeUnknown?: boolean) => ({
  isDir: true,
  size,
  count,
  sizeUnknown,
});
const file = (size: number) => ({ isDir: false, size });

describe("folderLine", () => {
  it("gives the count and the size", () => {
    expect(folderLine(dir(248, 1.82 * 1024 ** 3), items)).toBe(
      "248 öğe · 1.82 GiB"
    );
  });

  it("gives only the count for an empty folder or an unknown size", () => {
    expect(folderLine(dir(0, 0), items)).toBe("0 öğe");
    expect(folderLine(dir(12, 0, true), items)).toBe("12 öğe");
  });

  it("gives a dash without facts", () => {
    expect(folderLine(dir(undefined), items)).toBe("—");
  });
});

// The Info window used to add a folder's 4096-byte directory entry as its size.
describe("totalSize", () => {
  it("adds up files and the folders whose size is known", () => {
    expect(totalSize([file(10), dir(2, 100)])).toEqual({
      size: 110,
      partial: false,
    });
  });

  it("tells when a folder's size is not known", () => {
    expect(totalSize([file(10), dir(2, 0, true), dir(undefined)])).toEqual({
      size: 10,
      partial: true,
    });
    expect(knownSize(dir(undefined))).toBeNull();
  });
});
