import { describe, expect, it } from "vitest";
import { orderItems } from "../listingOrder";

const collator = new Intl.Collator("tr", {
  numeric: true,
  sensitivity: "base",
});
const item = (name: string, size: number, modified: string) => ({
  name,
  size,
  modified,
});
const items = [
  item("dosya10.txt", 30, "2026-10-03T00:00:00Z"),
  item("Ağaç.txt", 10, "2026-10-01T00:00:00Z"),
  item("dosya2.txt", 20, "2026-10-02T00:00:00Z"),
];
const names = (l: { name: string }[]) => l.map((i) => i.name);

// K111: the search results are ordered as the server orders a folder.
describe("orderItems", () => {
  it("orders by name A to Z when asc is off, numbers by value", () => {
    expect(
      names(orderItems(items, { by: "name", asc: false }, collator))
    ).toEqual(["Ağaç.txt", "dosya2.txt", "dosya10.txt"]);
    expect(
      names(orderItems(items, { by: "name", asc: true }, collator))
    ).toEqual(["dosya10.txt", "dosya2.txt", "Ağaç.txt"]);
  });

  it("orders by size and time, smallest and oldest first when asc is on", () => {
    expect(
      names(orderItems(items, { by: "size", asc: true }, collator))
    ).toEqual(["Ağaç.txt", "dosya2.txt", "dosya10.txt"]);
    expect(
      names(orderItems(items, { by: "modified", asc: false }, collator))
    ).toEqual(["dosya10.txt", "dosya2.txt", "Ağaç.txt"]);
  });

  it("leaves the items it is given as they are", () => {
    orderItems(items, { by: "size", asc: true }, collator);
    expect(names(items)).toEqual(["dosya10.txt", "Ağaç.txt", "dosya2.txt"]);
  });
});
