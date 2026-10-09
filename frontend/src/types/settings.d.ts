interface ISettings {
  createUserDir: boolean;
  minimumPasswordLength: number;
  userHomeBasePath: string;
  defaults: SettingsDefaults;
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
}

interface SettingsBranding {
  theme: UserTheme;
  disableUsedPercentage: boolean;
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
