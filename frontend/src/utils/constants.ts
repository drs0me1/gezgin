// The settings the server writes into the page as a JSON data block (Gezgin: the CSP allows no
// inline script).
const settings = JSON.parse(
  document.getElementById("gezgin-settings")?.textContent || "{}"
);

// Gezgin's brand is fixed: there is no instance name to set.
const name = "Gezgin";
const disableUsedPercentage: boolean = settings.DisableUsedPercentage;
const baseURL: string = settings.BaseURL;
const staticURL: string = settings.StaticURL;
const version: string = settings.Version;
// The version in the address makes a browser fetch a changed logo, cached for a day (Gezgin).
const logoURL = `${staticURL}/img/logo.svg?v=${encodeURIComponent(version)}`;
const theme: UserTheme = settings.Theme;
const enableThumbs: boolean = settings.EnableThumbs;
const resizePreview: boolean = settings.ResizePreview;
const tusSettings = settings.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;
// The port of the WebDAV shares (Gezgin); empty when they are off.
const webdavPort: string = settings.WebDAVPort || "";

export {
  name,
  disableUsedPercentage,
  baseURL,
  staticURL,
  logoURL,
  version,
  theme,
  enableThumbs,
  resizePreview,
  tusSettings,
  origin,
  tusEndpoint,
  webdavPort,
};
