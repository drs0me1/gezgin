// The files Gezgin opens as archives: ZIP, 7z, tar (plain or compressed) and RAR, a part of a
// set too (name.part2.rar, name.part2of3.rar, name.r00 to name.r99; later parts, as name.s00,
// are found from these). It matches the server's list.
const archiveName =
  /\.(zip|7z|rar|tar|tgz|tbz2?|txz|tzst|tar\.(gz|bz2|xz|zst))$|\.r\d\d$/i;

export const isArchive = (name: string) => archiveName.test(name);
