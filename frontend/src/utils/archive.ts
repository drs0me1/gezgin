// The files Gezgin opens as archives: ZIP, 7z, tar (plain or compressed) and RAR, a part of a
// set too (name.part2.rar, name.part2of3.rar, name.r00 to name.r99; later parts, as name.s00,
// are found from these), and a volume of a split ZIP, 7z or tar (name.zip.001). It matches the
// server's list.
const archiveName =
  /\.(zip|7z|rar|tar|tgz|tbz2?|txz|tzst|tar\.(gz|bz2|xz|zst))$|\.r\d\d$|\.(zip|7z|tar|tgz|tbz2?|txz|tzst|tar\.(gz|bz2|xz|zst))\.\d{3}$/i;

export const isArchive = (name: string) => archiveName.test(name);

// The archives Gezgin makes, with their extensions.
export const archiveFormats = {
  zip: ".zip",
  tar: ".tar",
  targz: ".tar.gz",
} as const;

export type ArchiveFormat = keyof typeof archiveFormats;

// The sizes a set's volumes can be split in, in bytes, as Windows and 7-Zip count a megabyte:
// for e-mail, for sharing, and below FAT32's 4 GiB file limit.
const MB = 1024 * 1024;
export const volumeSizes = {
  mb25: 25 * MB,
  mb100: 100 * MB,
  gb1: 1024 * MB,
  fat32: 4095 * MB,
} as const;

// volumeBytes is a custom volume size given in megabytes, or 0 when it is not a whole number
// of at least one.
export const volumeBytes = (megabytes: number) =>
  Number.isInteger(megabytes) && megabytes >= 1 ? megabytes * MB : 0;

// defaultArchiveName names an archive of items after the one item, a file without its
// extension, or after the folder they are in.
export function defaultArchiveName(
  items: { name: string; isDir: boolean }[],
  folder: string
) {
  if (items.length === 1) {
    const { name, isDir } = items[0];
    const dot = name.lastIndexOf(".");
    return isDir || dot <= 0 ? name : name.slice(0, dot);
  }
  return folder || "arsiv";
}
