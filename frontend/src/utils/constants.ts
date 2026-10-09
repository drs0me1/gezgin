// Gezgin's brand is fixed: there is no instance name to set.
const name = "Gezgin";
const disableUsedPercentage: boolean = window.FileBrowser.DisableUsedPercentage;
const baseURL: string = window.FileBrowser.BaseURL;
const staticURL: string = window.FileBrowser.StaticURL;
const version: string = window.FileBrowser.Version;
const logoURL = `${staticURL}/img/logo.svg`;
const authMethod = window.FileBrowser.AuthMethod;
const theme: UserTheme = window.FileBrowser.Theme;
const enableThumbs: boolean = window.FileBrowser.EnableThumbs;
const resizePreview: boolean = window.FileBrowser.ResizePreview;
const tusSettings = window.FileBrowser.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;
// The port of the WebDAV shares (Gezgin); empty when they are off.
const webdavPort: string = window.FileBrowser.WebDAVPort || "";

export {
  name,
  disableUsedPercentage,
  baseURL,
  staticURL,
  logoURL,
  version,
  authMethod,
  theme,
  enableThumbs,
  resizePreview,
  tusSettings,
  origin,
  tusEndpoint,
  webdavPort,
};
