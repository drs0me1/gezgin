<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!layoutStore.loading && settings !== null">
    <div class="column">
      <form class="card" @submit.prevent="save">
        <div class="card-title">
          <h2>{{ t("settings.globalSettings") }}</h2>
        </div>

        <div class="card-content">
          <p>
            <input type="checkbox" v-model="settings.createUserDir" />
            {{ t("settings.createUserDir") }}
          </p>

          <p>
            <label class="small">{{ t("settings.userHomeBasePath") }}</label>
            <input
              class="input input--block"
              type="text"
              v-model="settings.userHomeBasePath"
            />
          </p>

          <p>
            <label for="minimumPasswordLength">{{
              t("settings.minimumPasswordLength")
            }}</label>
            <vue-number-input
              controls
              v-model.number="settings.minimumPasswordLength"
              id="minimumPasswordLength"
              :min="minPasswordLength"
              :max="maxPasswordLength"
            />
          </p>

          <h3>{{ t("trash.title") }}</h3>
          <p class="small">{{ t("trash.keepDaysHelp") }}</p>
          <p>
            <label for="trashDays">{{ t("trash.keepDays") }}</label>
            <vue-number-input
              controls
              v-model.number="settings.trashDays"
              id="trashDays"
              :min="0"
              :max="3650"
            />
          </p>
          <p v-if="trashUsage" id="trashUsage">
            {{ t("trash.usage", { count: trashUsage.count }) }} ·
            {{ filesize(trashUsage.size) }}
            <button
              v-if="trashUsage.count > 0"
              type="button"
              class="button button--flat button--red"
              @click="emptyAllTrash"
            >
              {{
                confirmEmptyAll ? t("trash.confirmEmpty") : t("trash.emptyAll")
              }}
            </button>
          </p>

          <h3>{{ t("settings.rules") }}</h3>
          <p class="small">{{ t("settings.globalRules") }}</p>
          <rules v-model:rules="settings.rules" />

          <h3>{{ t("settings.appearance") }}</h3>

          <p>
            <label for="theme">{{ t("settings.themes.title") }}</label>
            <themes
              class="input input--block"
              v-model:theme="settings.branding.theme"
              id="theme"
            ></themes>
          </p>

          <p>
            <input
              type="checkbox"
              v-model="settings.branding.disableUsedPercentage"
              id="branding-used-disk"
            />
            {{ t("settings.disableUsedDiskPercentage") }}
          </p>

          <h3>{{ t("settings.tusUploads") }}</h3>

          <p class="small">{{ t("settings.tusUploadsHelp") }}</p>

          <div class="tusConditionalSettings">
            <p>
              <label for="tus-chunkSize">{{
                t("settings.tusUploadsChunkSize")
              }}</label>
              <input
                class="input input--block"
                type="text"
                v-model="chunkSizeText"
                id="tus-chunkSize"
              />
            </p>

            <p>
              <label for="tus-retryCount">{{
                t("settings.tusUploadsRetryCount")
              }}</label>
              <vue-number-input
                controls
                v-model.number="settings.tus.retryCount"
                id="tus-retryCount"
                :min="0"
                :max="maxRetryCount"
              />
            </p>
          </div>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="t('buttons.update')"
          />
        </div>
      </form>
    </div>

    <div class="column">
      <form class="card" @submit.prevent="save">
        <div class="card-title">
          <h2>{{ t("settings.userDefaults") }}</h2>
        </div>

        <div class="card-content">
          <p class="small">{{ t("settings.defaultUserDescription") }}</p>

          <user-form
            :isNew="false"
            :isDefault="true"
            v-model:user="settings.defaults"
          />
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="t('buttons.update')"
          />
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { settings as api, trash as trashApi } from "@/api";
import { filesize } from "@/utils";
import { StatusError } from "@/api/utils";
import Rules from "@/components/settings/Rules.vue";
import Themes from "@/components/settings/Themes.vue";
import UserForm from "@/components/settings/UserForm.vue";
import { useLayoutStore } from "@/stores/layout";
import { formatSize, parseSize } from "@/utils/size";
import { getTheme, setTheme } from "@/utils/theme";
import Errors from "@/views/Errors.vue";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The bounds the server holds the settings to (settings.Validate).
const minPasswordLength = 8;
const maxPasswordLength = 32;
const minChunkSize = 1024 ** 2;
const maxChunkSize = 1024 ** 3;
const maxRetryCount = 20;

const error = ref<StatusError | null>(null);
const originalSettings = ref<ISettings | null>(null);
const settings = ref<ISettings | null>(null);
const chunkSizeText = ref<string>("");
const trashUsage = ref<{ count: number; size: number } | null>(null);
const confirmEmptyAll = ref<boolean>(false);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const { t } = useI18n();

const layoutStore = useLayoutStore();

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
  if (settings.value === null || originalSettings.value === null) return;

  // The text shown rounds the size; left as shown, it keeps the size as it was.
  const chunkSize =
    chunkSizeText.value === formatSize(settings.value.tus.chunkSize)
      ? settings.value.tus.chunkSize
      : parseSize(chunkSizeText.value);
  if (chunkSize === null) {
    $showError(t("settings.errors.chunkSizeUnreadable"));
    return;
  }
  const newSettings: ISettings = {
    ...settings.value,
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
  if (
    interfaceSettings(newSettings) !== interfaceSettings(originalSettings.value)
  ) {
    if (newSettings.branding.theme !== getTheme()) {
      setTheme(newSettings.branding.theme);
    }
    window.setTimeout(() => window.location.reload(), 1000);
    return;
  }
  originalSettings.value = JSON.parse(JSON.stringify(newSettings));
};

const loadTrashUsage = async () => {
  try {
    trashUsage.value = await trashApi.usage();
  } catch (e: any) {
    $showError(e);
  }
};

// Every bin is emptied only on a second click.
const emptyAllTrash = async () => {
  if (!confirmEmptyAll.value) {
    confirmEmptyAll.value = true;
    return;
  }
  confirmEmptyAll.value = false;
  try {
    await trashApi.emptyAll();
    $showSuccess(t("trash.emptied"));
  } catch (e: any) {
    $showError(e);
  }
  await loadTrashUsage();
};

// Define Hooks

onMounted(async () => {
  try {
    layoutStore.loading = true;
    const original: ISettings = await api.get();

    originalSettings.value = original;
    settings.value = JSON.parse(JSON.stringify(original));
    chunkSizeText.value = formatSize(original.tus.chunkSize);
    await loadTrashUsage();
  } catch (err) {
    if (err instanceof Error) {
      error.value = err;
    }
  } finally {
    layoutStore.loading = false;
  }
});
</script>
