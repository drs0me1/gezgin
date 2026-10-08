interface ISettings {
  signup: boolean;
  createUserDir: boolean;
  hideLoginButton: boolean;
  minimumPasswordLength: number;
  userHomeBasePath: string;
  defaults: SettingsDefaults;
  authMethod: string;
  rules: any[];
  branding: SettingsBranding;
  tus: SettingsTus;
  trashDays: number;
}

interface SettingsDefaults {
  scope: string;
  locale: string;
  viewMode: ViewModeType;
  singleClick: boolean;
  redirectAfterCopyMove: boolean;
  sorting: Sorting;
  perm: Permissions;
  hideDotfiles: boolean;
  dateFormat: boolean;
  aceEditorTheme: string;
}

interface SettingsBranding {
  name: string;
  disableExternal: boolean;
  disableUsedPercentage: boolean;
  files: string;
  theme: UserTheme;
  color: string;
}

interface SettingsTus {
  chunkSize: number;
  retryCount: number;
}

interface SettingsUnit {
  KB: number;
  MB: number;
  GB: number;
  TB: number;
}
