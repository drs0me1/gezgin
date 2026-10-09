// The favourites as the server lists them (Gezgin): a folder's listing, of items from anywhere.
interface IFavoritesListing {
  items: ResourceItem[];
  numDirs: number;
  numFiles: number;
  sorting: Sorting;
}
