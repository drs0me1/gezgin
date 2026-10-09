// Sizes as the settings screen shows and reads them: 10MB, 1.5 GB, 512 KiB or a plain number of
// bytes. Units are binary (1 MB = 1024 KB), as File Browser had them.

const units: Record<string, number> = {
  "": 1,
  K: 1024,
  M: 1024 ** 2,
  G: 1024 ** 3,
  T: 1024 ** 4,
};

// parseSize reads a size in bytes, or returns null for text it cannot read. A comma is taken as
// the decimal separator too ("1,5 GB").
export function parseSize(input: string): number | null {
  const match = input
    .trim()
    .replace(",", ".")
    .match(/^(\d+(?:\.\d+)?)\s*([KMGT]?)(?:i?B)?$/i);
  if (!match) return null;
  return Math.round(parseFloat(match[1]) * units[match[2].toUpperCase()]);
}

// formatSize writes a size in bytes with the largest unit that keeps it at 1 or more.
export function formatSize(bytes: number): string {
  const names = ["B", "KB", "MB", "GB", "TB"];
  let size = bytes;
  let i = 0;
  while (size >= 1024 && i < names.length - 1) {
    size /= 1024;
    i++;
  }
  return `${+size.toFixed(2)}${names[i]}`;
}
