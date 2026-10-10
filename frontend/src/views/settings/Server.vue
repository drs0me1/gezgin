<template>
  <errors v-if="error" :errorCode="error.status" />
  <div v-else-if="!layoutStore.loading && info">
    <div class="settings-card">
      <h3>Gezgin</h3>
      <p class="setting-help">{{ t("settings.server.help") }}</p>
      <setting-row :label="t('settings.server.version')">
        <span class="setting-value">{{ info.version }}</span>
      </setting-row>
      <setting-row :label="t('settings.server.webdav')">
        <span class="setting-value">
          {{
            info.webdavPort
              ? t("settings.server.webdavOn", { port: info.webdavPort })
              : t("settings.server.off")
          }}
        </span>
      </setting-row>
      <setting-row :label="t('settings.server.thumbnails')">
        <span class="setting-value">
          {{
            info.thumbnails ? t("settings.server.on") : t("settings.server.off")
          }}
        </span>
      </setting-row>
      <setting-row
        :label="t('settings.server.session')"
        :help="t('settings.server.sessionHelp')"
      >
        <span class="setting-value">{{ duration(info.sessionSeconds) }}</span>
      </setting-row>
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.server.storage") }}</h3>
      <setting-row v-if="disk" :label="t('settings.server.files')">
        <span class="setting-value">
          {{ filesize(disk.used) }} / {{ filesize(disk.total) }}
        </span>
      </setting-row>
      <setting-row :label="t('settings.server.database')">
        <span class="setting-value">{{ filesize(info.databaseSize) }}</span>
      </setting-row>
      <setting-row
        :label="t('settings.server.cache')"
        :help="
          info.cache
            ? t('settings.server.cacheHelp', {
                count: info.cache.count,
                size: filesize(info.cache.size),
              })
            : t('settings.server.cacheOff')
        "
        stack
      >
        <button
          v-if="info.cache && info.cache.count > 0"
          type="button"
          class="button button--flat"
          :disabled="clearing"
          @click="clearCache"
        >
          {{ t("settings.server.clearCache") }}
        </button>
      </setting-row>
      <setting-row v-if="trashUsage" :label="t('settings.server.trash')">
        <span class="setting-value">
          {{
            t(
              "trash.usageShort",
              { count: trashUsage.count, size: filesize(trashUsage.size) },
              trashUsage.count
            )
          }}
        </span>
      </setting-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { files, server, trash } from "@/api";
import type { ServerInfo } from "@/api/server";
import { StatusError } from "@/api/utils";
import SettingRow from "@/components/settings/SettingRow.vue";
import { useLayoutStore } from "@/stores/layout";
import { filesize } from "@/utils";
import Errors from "@/views/Errors.vue";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The server page (Gezgin, K146, K147): what the container runs with, set in Konsol and only
// shown here, the sizes Gezgin keeps beside the files, and the thumbnail cache, which it clears.
const error = ref<StatusError | null>(null);
const info = ref<ServerInfo | null>(null);
const disk = ref<{ used: number; total: number } | null>(null);
const trashUsage = ref<{ count: number; size: number } | null>(null);
const clearing = ref<boolean>(false);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const layoutStore = useLayoutStore();
const { t } = useI18n();

const duration = (seconds: number) => {
  if (seconds > 0 && seconds % 86400 === 0)
    return t("settings.server.days", seconds / 86400);
  if (seconds > 0 && seconds % 3600 === 0)
    return t("settings.server.hours", seconds / 3600);
  return t("settings.server.minutes", Math.round(seconds / 60));
};

const clearCache = async () => {
  clearing.value = true;
  try {
    await server.clearCache();
    $showSuccess(t("settings.server.cacheCleared"));
    info.value = await server.get();
  } catch (e: any) {
    $showError(e);
  } finally {
    clearing.value = false;
  }
};

onMounted(async () => {
  layoutStore.loading = true;
  try {
    info.value = await server.get();
    // The disk and the trash are shown as elsewhere; without them the page still stands.
    const [usage, bins] = await Promise.allSettled([
      files.usage("/", new AbortController().signal),
      trash.usage(),
    ]);
    if (usage.status === "fulfilled") disk.value = usage.value;
    if (bins.status === "fulfilled") trashUsage.value = bins.value;
  } catch (err) {
    if (err instanceof Error) {
      error.value = err as StatusError;
    }
  } finally {
    layoutStore.loading = false;
  }
});
</script>
