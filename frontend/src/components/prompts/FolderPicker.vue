<template>
  <div class="card floating" id="folder-picker">
    <div class="card-title">
      <h2>{{ t("settings.access.pickTitle") }}</h2>
    </div>
    <div class="card-content">
      <file-list
        :start="props.start"
        @update:selected="(val: string) => (dest = val)"
      />
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
        @click="choose"
        :aria-label="t('buttons.select')"
        :title="t('buttons.select')"
      >
        {{ t("buttons.select") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import FileList from "./FileList.vue";

// A folder picked in the tree for a user's access (Gezgin, K148): the folder selected, or the
// one open when none is.
const { t } = useI18n();
const layoutStore = useLayoutStore();
const props: { start?: Resource } = layoutStore.currentPrompt?.props ?? {};
const dest = ref<string | null>(null);

const choose = () => {
  const confirm = layoutStore.currentPrompt?.confirm;
  layoutStore.closeHovers();
  if (dest.value !== null) confirm?.(dest.value);
};
</script>
