interface ResourceBase {
  path: string;
  name: string;
  size: number;
  extension: string;
  modified: string; // ISO 8601 datetime
  mode: number;
  isDir: boolean;
  isSymlink: boolean;
  type: ResourceType;
  url: string;
  // A folder's item count, and whether its size is unknown, when a listing asked for them
  // (Gezgin).
  count?: number;
  sizeUnknown?: boolean;
  // Marks the server sets as it lists (Gezgin, K114-K116): a favourite, and the shares in force.
  favorite?: boolean;
  sharedLinks?: number;
  sharedDav?: number;
}

interface Resource extends ResourceBase {
  items: ResourceItem[];
  numDirs: number;
  numFiles: number;
  sorting: Sorting;
  hash?: string;
  token?: string;
  index: number;
  subtitles?: string[];
  content?: string;
  rawContent?: ArrayBuffer;
  encoding?: string;
  version?: string;
  // The favourites page (Gezgin), whose items are shortcuts to items elsewhere.
  favorites?: boolean;
  // The trash page (Gezgin), whose items no path reaches.
  trash?: boolean;
  // The search results (Gezgin, K111), whose items are shortcuts to items elsewhere.
  search?: boolean;
  // The shares page (Gezgin, K130), whose items are shares: each a shortcut to its item.
  shares?: boolean;
}

interface ResourceItem extends ResourceBase {
  index: number;
  subtitles?: string[];
  // Where a shortcut's item lies, or where a trashed item was (Gezgin).
  location?: string;
  // A trashed item's id (Gezgin).
  id?: string;
}

type ResourceType =
  | "dir"
  | "video"
  | "audio"
  | "image"
  | "pdf"
  | "text"
  | "blob"
  | "archive"
  | "textImmutable";

type DownloadFormat =
  | "zip"
  | "tar"
  | "targz"
  | "tarbz2"
  | "tarxz"
  | "tarlz4"
  | "tarsz"
  | null;

interface ClipItem {
  from: string;
  name: string;
  size?: number;
  isDir?: boolean;
  modified?: string;
}

interface BreadCrumb {
  name: string;
  url: string;
}

interface ConflictingItem {
  lastModified: number | string | undefined;
  size: number | undefined;
}

interface ConflictingResource {
  index: number;
  name: string;
  origin: ConflictingItem;
  dest: ConflictingItem;
  checked: Array<"origin" | "dest", "origin-resume">;
  isSmallerOnServer?: boolean;
}

interface CsvData {
  headers: string[];
  rows: string[][];
}

interface RecursiveEntry {
  path: string;
  name: string;
  size: number;
  modified: string;
  isDir: boolean;
}
