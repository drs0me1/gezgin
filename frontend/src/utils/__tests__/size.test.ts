import { describe, expect, it } from "vitest";
import { formatSize, parseSize } from "../size";

describe("parseSize", () => {
  it("reads sizes with or without a unit or a space", () => {
    expect(parseSize("10MB")).toBe(10 * 1024 ** 2);
    expect(parseSize("10 MB")).toBe(10 * 1024 ** 2);
    expect(parseSize("10m")).toBe(10 * 1024 ** 2);
    expect(parseSize("512 KiB")).toBe(512 * 1024);
    expect(parseSize("1.5G")).toBe(1.5 * 1024 ** 3);
    expect(parseSize("1,5 GB")).toBe(1.5 * 1024 ** 3);
    expect(parseSize("1048576")).toBe(1024 ** 2);
    expect(parseSize(" 1T ")).toBe(1024 ** 4);
  });

  it("rounds to whole bytes", () => {
    expect(parseSize("1.3K")).toBe(1331);
  });

  // The form used to read text it could not parse as 1 MB, without a word.
  it("refuses text it cannot read", () => {
    for (const text of ["", "MB", "ten MB", "10 XB", "-5MB", "1.2.3MB"]) {
      expect(parseSize(text)).toBeNull();
    }
  });
});

describe("formatSize", () => {
  it("writes the largest unit, and reads back what it wrote", () => {
    expect(formatSize(10 * 1024 ** 2)).toBe("10MB");
    expect(formatSize(1024 ** 3)).toBe("1GB");
    expect(formatSize(1536 * 1024)).toBe("1.5MB");
    expect(formatSize(1000)).toBe("1000B");
    for (const n of [1024 ** 2, 10 * 1024 ** 2, 1024 ** 3]) {
      expect(parseSize(formatSize(n))).toBe(n);
    }
  });
});
