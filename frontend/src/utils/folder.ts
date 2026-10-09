import { filesize } from "@/utils";

// A folder's facts come with a listing asked for them (Gezgin, K84): how many items it shows, and
// its size, unless the walk stopped short.
type Sized = Pick<ResourceBase, "isDir" | "size" | "count" | "sizeUnknown">;

// knownSize is an item's size, or null for a folder whose size is not known.
export function knownSize(item: Sized): number | null {
  if (!item.isDir) return item.size;
  return item.count !== undefined && !item.sizeUnknown ? item.size : null;
}

// totalSize adds up the sizes it knows; partial tells that some it does not.
export function totalSize(items: Sized[]): { size: number; partial: boolean } {
  let size = 0;
  let partial = false;
  for (const item of items) {
    const s = knownSize(item);
    if (s === null) partial = true;
    else size += s;
  }
  return { size, partial };
}

// folderLine is a folder's line in a listing: "248 öğe · 1.82 GiB", only the count when the size
// is not known or the folder is empty, and "—" without facts.
export function folderLine(
  item: Sized,
  items: (count: number) => string
): string {
  if (item.count === undefined) return "—";
  const size = knownSize(item);
  if (size === null || item.count === 0) return items(item.count);
  return `${items(item.count)} · ${filesize(size)}`;
}
