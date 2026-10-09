interface ITrashItem {
  id: string;
  name: string;
  origin: string;
  deleted: string;
  size: number;
  isDir: boolean;
  // A file's type by its name, and a folder's item count (Gezgin).
  type?: ResourceType;
  count?: number;
}

interface ITrashList {
  items: ITrashItem[];
  count: number;
  size: number;
}
