interface ITrashItem {
  id: string;
  name: string;
  origin: string;
  deleted: string;
  size: number;
  isDir: boolean;
}

interface ITrashList {
  items: ITrashItem[];
  count: number;
  size: number;
}
