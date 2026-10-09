<template>
  <errors v-if="error" :errorCode="error.status" />
  <div v-else-if="!layoutStore.loading && settings !== null">
    <div class="settings-card">
      <h3>{{ t("settings.security") }}</h3>
      <setting-row
        :label="t('settings.minimumPasswordLength')"
        :help="
          t('settings.minimumPasswordLengthHelp', {
            min: minPasswordLength,
            max: maxPasswordLength,
          })
        "
      >
        <vue-number-input
          controls
          size="small"
          v-model.number="settings.minimumPasswordLength"
          :min="minPasswordLength"
          :max="maxPasswordLength"
        />
      </setting-row>
    </div>

    <div class="settings-card">
      <h3>{{ t("trash.title") }}</h3>
      <setting-row :label="t('trash.keepDays')" :help="t('trash.keepDaysHelp')">
        <vue-number-input
          controls
          size="small"
          v-model.number="settings.trashDays"
          :min="0"
          :max="3650"
        />
      </setting-row>
      <setting-row
        v-if="trashUsage"
        :label="t('trash.allUsers')"
        :help="
          t('trash.usageShort', {
            count: trashUsage.count,
            size: filesize(trashUsage.size),
          })
        "
        stack
      >
        <button
          v-if="trashUsage.count > 0"
          type="button"
          class="button button--flat button--red"
          @click="emptyAllTrash"
        >
          {{ t("trash.emptyAll") }}
        </button>
      </setting-row>
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.appearance") }}</h3>
      <setting-row :label="t('settings.themes.title')">
        <themes v-model:theme="settings.branding.theme" />
      </setting-row>
      <setting-row :label="t('settings.showUsedDisk')">
        <toggle-switch
          :model-value="!settings.branding.disableUsedPercentage"
          :label="t('settings.showUsedDisk')"
          @update:model-value="
            (show: boolean) =>
              (settings!.branding.disableUsedPercentage = !show)
          "
        />
      </setting-row>
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.newUsers") }}</h3>
      <p class="setting-help">{{ t("settings.newUsersHelp") }}</p>
      <h4>{{ t("settings.access.title") }}</h4>
      <scope-choice
        v-model:mode="access.mode"
        v-model:folder="access.folder"
        :own-path="
          homeDir(settings.userHomeBasePath, t('settings.access.name'))
        "
        name="default-scope"
      />
      <setting-row
        v-if="access.mode === 'own'"
        :label="t('settings.homesFolder')"
        label-for="homes"
        stack
      >
        <input
          id="homes"
          class="input"
          type="text"
          v-model="settings.userHomeBasePath"
        />
      </setting-row>
      <setting-row :label="t('settings.language')" label-for="default-locale">
        <languages
          id="default-locale"
          class="input"
          v-model:locale="settings.defaults.locale"
        />
      </setting-row>
      <h4>{{ t("settings.permissions") }}</h4>
      <permissions v-model:perm="settings.defaults.perm" :is-default="true" />
    </div>

    <div class="settings-card">
      <details>
        <summary>
          <icon name="chevron_right" />{{ t("settings.advancedGlobal") }}
        </summary>
        <setting-row
          :label="t('settings.chunkSize')"
          :help="t('settings.chunkSizeHelp', { max: maxChunkMB })"
        >
          <vue-number-input
            controls
            size="small"
            v-model.number="chunkSizeMB"
            :min="1"
            :max="maxChunkMB"
            :step="1"
          />
          <span class="setting-unit">MB</span>
        </setting-row>
        <setting-row
          :label="t('settings.retryCount')"
          :help="t('settings.retryCountHelp', { max: maxRetryCount })"
        >
          <vue-number-input
            controls
            size="small"
            v-model.number="settings.tus.retryCount"
            :min="0"
            :max="maxRetryCount"
          />
        </setting-row>
        <h4>{{ t("settings.globalRulesTitle") }}</h4>
        <p class="setting-help">{{ t("settings.globalRules") }}</p>
        <rules v-model:rules="settings.rules" />
      </details>
    </div>

    <save-bar :visible="dirty" @cancel="reset" @save="save" />
  </div>
</template>

<script setup lang="ts">
import { settings as api, trash as trashApi } from "@/api";
import { StatusError } from "@/api/utils";
import Icon from "@/components/Icon.vue";
import Languages from "@/components/settings/Languages.vue";
import Permissions from "@/components/settings/Permissions.vue";
import Rules from "@/components/settings/Rules.vue";
import SaveBar from "@/components/settings/SaveBar.vue";
import ScopeChoice from "@/components/settings/ScopeChoice.vue";
import SettingRow from "@/components/settings/SettingRow.vue";
import Themes from "@/components/settings/Themes.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { useLayoutStore } from "@/stores/layout";
import { filesize } from "@/utils";
import { homeDir, scopeMode, type ScopeMode } from "@/utils/scope";
import { getTheme, setTheme } from "@/utils/theme";
import Errors from "@/views/Errors.vue";
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The general settings (Gezgin, K150-K153): security, the trash, the look, what new users get,
// and folded under "Gelişmiş" the chunked uploads and the global rules; one save for the page.

// The bounds the server holds the settings to (settings.Validate).
const minPasswordLength = 8;
const maxPasswordLength = 32;
const MB = 1024 ** 2;
const minChunkSize = MB;
const maxChunkSize = 1024 ** 3;
const maxChunkMB = maxChunkSize / MB;
const maxRetryCount = 20;

const error = ref<StatusError | null>(null);
const original = ref<ISettings | null>(null);
const settings = ref<ISettings | null>(null);
// The chunk size in whole MB, stepped by one (K156).
const chunkSizeMB = ref<number>(10);
const toMB = (bytes: number) => Math.max(1, Math.round(bytes / MB));
const access = ref<{ mode: ScopeMode; folder: string }>({
  mode: "all",
  folder: "",
});
const trashUsage = ref<{ count: number; size: number } | null>(null);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const { t } = useI18n();

const layoutStore = useLayoutStore();

// What new users get is "make the home folder" for their own folder, else the default scope.
const accessOf = (s: ISettings) =>
  s.createUserDir
    ? { mode: "own" as ScopeMode, folder: "" }
    : scopeMode(s.defaults.scope, null);

const load = (s: ISettings) => {
  original.value = s;
  settings.value = JSON.parse(JSON.stringify(s));
  chunkSizeMB.value = toMB(s.tus.chunkSize);
  access.value = accessOf(s);
};

const reset = () => {
  if (original.value) load(original.value);
};

const dirty = computed(
  () =>
    settings.value !== null &&
    original.value !== null &&
    (JSON.stringify(settings.value) !== JSON.stringify(original.value) ||
      chunkSizeMB.value !== toMB(original.value.tus.chunkSize) ||
      JSON.stringify(access.value) !== JSON.stringify(accessOf(original.value)))
);

// check returns what is wrong with the settings, as the server would say it, or null.
const check = (s: ISettings): string | null => {
  const length = s.minimumPasswordLength;
  if (
    !Number.isInteger(length) ||
    length < minPasswordLength ||
    length > maxPasswordLength
  ) {
    return t("settings.errors.passwordLength", {
      min: minPasswordLength,
      max: maxPasswordLength,
    });
  }
  const chunkSize = s.tus.chunkSize;
  if (chunkSize < minChunkSize || chunkSize > maxChunkSize) {
    return t("settings.errors.chunkSize", {
      min: filesize(minChunkSize),
      max: filesize(maxChunkSize),
    });
  }
  const retries = s.tus.retryCount;
  if (!Number.isInteger(retries) || retries < 0 || retries > maxRetryCount) {
    return t("settings.errors.retryCount", { min: 0, max: maxRetryCount });
  }
  return null;
};

// The interface reads these when the page loads; a change to them reloads it.
const interfaceSettings = (s: ISettings) => JSON.stringify([s.branding, s.tus]);

const save = async () => {
  if (settings.value === null || original.value === null) return;

  // The MB shown round the size; left as shown, it keeps the size as it was.
  const chunkSize =
    chunkSizeMB.value === toMB(settings.value.tus.chunkSize)
      ? settings.value.tus.chunkSize
      : chunkSizeMB.value * MB;
  if (access.value.mode === "folder" && access.value.folder === "") {
    $showError(t("settings.access.noFolder"));
    return;
  }
  const newSettings: ISettings = {
    ...settings.value,
    createUserDir: access.value.mode === "own",
    defaults: {
      ...settings.value.defaults,
      scope: access.value.mode === "folder" ? access.value.folder : ".",
    },
    tus: { ...settings.value.tus, chunkSize },
  };
  const problem = check(newSettings);
  if (problem !== null) {
    $showError(problem);
    return;
  }

  try {
    await api.update(newSettings);
  } catch (e: any) {
    $showError(e);
    return;
  }

  $showSuccess(t("settings.settingsUpdated"));
  if (interfaceSettings(newSettings) !== interfaceSettings(original.value)) {
    if (newSettings.branding.theme !== getTheme()) {
      setTheme(newSettings.branding.theme);
    }
    window.setTimeout(() => window.location.reload(), 1000);
    return;
  }
  load(JSON.parse(JSON.stringify(newSettings)));
};

const loadTrashUsage = async () => {
  try {
    trashUsage.value = await trashApi.usage();
  } catch (e: any) {
    $showError(e);
  }
};

// Every user's trash is emptied for good, so it asks first.
const emptyAllTrash = () => {
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("trash.emptyAllConfirm", {
        count: trashUsage.value?.count ?? 0,
      }),
      confirm: t("trash.emptyAll"),
      danger: true,
    },
    confirm: async () => {
      layoutStore.closeHovers();
      try {
        await trashApi.emptyAll();
        $showSuccess(t("trash.emptied"));
      } catch (e: any) {
        $showError(e);
      }
      await loadTrashUsage();
    },
  });
};

onMounted(async () => {
  try {
    layoutStore.loading = true;
    load(await api.get());
    await loadTrashUsage();
  } catch (err) {
    if (err instanceof Error) {
      error.value = err as StatusError;
    }
  } finally {
    layoutStore.loading = false;
  }
});
</script>
