<template>
  <div class="card floating" id="archive">
    <div class="card-title">
      <h2>{{ t("prompts.archive") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ t("prompts.archiveMessage") }}</p>
      <ul class="extract-names">
        <li v-for="name in shownNames" :key="name">
          <code>{{ name }}</code>
        </li>
        <li v-if="names.length > shownNames.length">
          {{
            t("prompts.archiveMore", {
              count: names.length - shownNames.length,
            })
          }}
        </li>
      </ul>

      <p>
        <label for="archive-name">{{ t("prompts.archiveName") }}</label>
      </p>
      <div class="archive-name">
        <input
          id="archive-name"
          class="input input--block"
          type="text"
          autocomplete="off"
          v-model="name"
          @keyup.enter="submit"
        />
        <span>{{ ext }}</span>
      </div>

      <p>
        <label for="archive-format">{{ t("prompts.archiveFormat") }}</label>
      </p>
      <select id="archive-format" class="input input--block" v-model="format">
        <option value="zip">{{ t("prompts.archiveFormatZip") }}</option>
        <option value="tar">{{ t("prompts.archiveFormatTar") }}</option>
        <option value="targz">{{ t("prompts.archiveFormatTarGz") }}</option>
      </select>
      <p v-if="format === 'zip'" class="small">
        {{ t("prompts.archiveFormatZipHint") }}
      </p>

      <p>
        <label for="archive-split">{{ t("prompts.archiveSplit") }}</label>
      </p>
      <select id="archive-split" class="input input--block" v-model="split">
        <option value="none">{{ t("prompts.archiveSplitNone") }}</option>
        <option value="mb25">25 MB</option>
        <option value="mb100">100 MB</option>
        <option value="gb1">1 GB</option>
        <option value="fat32">{{ t("prompts.archiveSplitFat32") }}</option>
        <option value="custom">{{ t("prompts.archiveSplitCustom") }}</option>
      </select>
      <template v-if="split === 'custom'">
        <p>
          <label for="archive-split-size">{{
            t("prompts.archiveSplitSize")
          }}</label>
        </p>
        <input
          id="archive-split-size"
          class="input input--block"
          type="number"
          min="1"
          step="1"
          v-model.number="megabytes"
          @keyup.enter="submit"
        />
      </template>
      <p v-if="split !== 'none'" class="small">
        {{
          t("prompts.archiveSplitHelp", {
            name: (name.trim() || "arsiv") + ext,
          })
        }}
      </p>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers"
        :aria-label="t('buttons.cancel')"
        :title="t('buttons.cancel')"
      >
        {{ t("buttons.cancel") }}
      </button>
      <button
        id="focus-prompt"
        class="button button--flat"
        @click="submit"
        :disabled="!ready"
        :aria-label="t('buttons.archiveCreate')"
        :title="t('buttons.archiveCreate')"
      >
        {{ t("buttons.archiveCreate") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useArchiveStore } from "@/stores/archive";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import {
  archiveFormats,
  defaultArchiveName,
  volumeBytes,
  volumeSizes,
  type ArchiveFormat,
} from "@/utils/archive";

// Archive starts a job that packs the selected items into an archive beside them, on the
// server (Gezgin).

const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const archiveStore = useArchiveStore();
const $showError = inject<IToastError>("$showError")!;

const items = computed(() =>
  fileStore.selected
    .map((i) => fileStore.req?.items[i])
    .filter((item): item is ResourceItem => item !== undefined)
);
const names = computed(() => items.value.map((item) => item.name));
const shownNames = computed(() => names.value.slice(0, 5));

const name = ref<string>(
  defaultArchiveName(items.value, fileStore.req?.name ?? "")
);
const format = ref<ArchiveFormat>("zip");
const split = ref<"none" | "custom" | keyof typeof volumeSizes>("none");
const megabytes = ref<number>(500);
const busy = ref<boolean>(false);

const ext = computed(() => archiveFormats[format.value]);
const volume = computed(() => {
  if (split.value === "none") return 0;
  if (split.value === "custom") return volumeBytes(megabytes.value);
  return volumeSizes[split.value];
});
const ready = computed(
  () =>
    !busy.value &&
    items.value.length > 0 &&
    name.value.trim() !== "" &&
    (split.value === "none" || volume.value > 0)
);

const submit = async () => {
  if (!ready.value) return;
  busy.value = true;
  try {
    await archiveStore.create(
      items.value.map((item) => item.path),
      name.value.trim(),
      format.value,
      volume.value
    );
    layoutStore.closeHovers();
  } catch (e: any) {
    $showError(e);
  } finally {
    busy.value = false;
  }
};
</script>
