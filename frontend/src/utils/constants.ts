const name: string = window.FileBrowser.Name || "File Browser";
const disableExternal: boolean = window.FileBrowser.DisableExternal;
const disableUsedPercentage: boolean = window.FileBrowser.DisableUsedPercentage;
const baseURL: string = window.FileBrowser.BaseURL;
const staticURL: string = window.FileBrowser.StaticURL;
const version: string = window.FileBrowser.Version;
const logoURL = `${staticURL}/img/logo.svg`;
const authMethod = window.FileBrowser.AuthMethod;
const logoutPage: string = window.FileBrowser.LogoutPage;
const loginPage: boolean = window.FileBrowser.LoginPage;
const theme: UserTheme = window.FileBrowser.Theme;
const enableThumbs: boolean = window.FileBrowser.EnableThumbs;
const resizePreview: boolean = window.FileBrowser.ResizePreview;
const tusSettings = window.FileBrowser.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;
const hideLoginButton = window.FileBrowser.HideLoginButton;
// The port of the WebDAV shares (Gezgin); empty when they are off.
const webdavPort: string = window.FileBrowser.WebDAVPort || "";

export {
  name,
  disableExternal,
  disableUsedPercentage,
  baseURL,
  staticURL,
  logoURL,
  version,
  authMethod,
  logoutPage,
  loginPage,
  theme,
  enableThumbs,
  resizePreview,
  tusSettings,
  origin,
  tusEndpoint,
  hideLoginButton,
  webdavPort,
};
