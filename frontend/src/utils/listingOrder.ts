// The order of a listing the browser builds itself, the search results (Gezgin, K111): the
// server's order of a folder (files/listing.go ApplySort), so that both read alike.

type Orderable = Pick<ResourceBase, "name" | "size" | "modified">;

// orderItems orders items by the user's sorting: by name in the language's order with numbers by
// value, A to Z when "asc" is off (File Browser's name order is reversed); by size or time, the
// smallest or oldest first when "asc" is on. Equal items keep a name order.
export function orderItems<T extends Orderable>(
  items: T[],
  sorting: Sorting | undefined,
  collator: Intl.Collator
): T[] {
  const asc = sorting?.asc ?? false;
  const byName = (a: T, b: T) => collator.compare(a.name, b.name);
  let compare: (a: T, b: T) => number;
  switch (sorting?.by) {
    case "size":
      compare = (a, b) =>
        (asc ? a.size - b.size : b.size - a.size) || byName(a, b);
      break;
    case "modified":
      compare = (a, b) => {
        const d = Date.parse(a.modified) - Date.parse(b.modified);
        return (asc ? d : -d) || byName(a, b);
      };
      break;
    default:
      compare = (a, b) => (asc ? -byName(a, b) : byName(a, b));
  }
  return [...items].sort(compare);
}
