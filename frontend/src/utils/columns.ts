import css from "@/utils/css";

// fitColumns sizes the tiles of a folder view to the window, as many columns of about width
// pixels as the page holds; the folder, favourites, trash, search and shares pages share the
// rule, so that their tiles are alike (Gezgin, K130).
export function fitColumns(width = 280) {
  const rule = css(["#listing.mosaic .item", ".mosaic#listing .item"]);
  if (rule === null) return;
  const main = document.querySelector("main")?.offsetWidth ?? 0;
  const columns = Math.max(1, Math.floor(main / width));
  rule.style.width = `calc(${100 / columns}% - 1em)`;
}
