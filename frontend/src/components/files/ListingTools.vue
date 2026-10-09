<template>
  <div class="listing-tools">
    <button
      class="action"
      @click="switchView"
      :aria-label="t('buttons.switchView')"
      :title="t('buttons.switchView')"
    >
      <icon :name="viewIcon" />
    </button>
    <button
      class="select-toggle"
      :class="{ active: fileStore.multiple }"
      :aria-pressed="fileStore.multiple"
      @click="toggleSelection"
    >
      {{ fileStore.multiple ? t("buttons.done") : t("buttons.select") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { users } from "@/api";
import Icon from "@/components/Icon.vue";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { computed, inject, onBeforeUnmount, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";

// The view and the selection of a folder view, at the right of its path or top line (Gezgin,
// K125, K129): one button for the view, showing the one it goes to, and "Seç", which puts a tick
// circle on every item, a click then ticking it; "Bitti" ends it and clears the selection.

const { t } = useI18n();
const route = useRoute();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;

const viewIcon = computed(() => {
  const icons = {
    list: "view_module",
    mosaic: "grid_view",
    "mosaic gallery": "view_list",
  };
  return icons[authStore.user?.viewMode ?? "list"];
});

const switchView = () => {
  layoutStore.closeHovers();
  const modes = {
    list: "mosaic",
    mosaic: "mosaic gallery",
    "mosaic gallery": "list",
  };
  const data = {
    id: authStore.user?.id,
    viewMode: (modes[authStore.user?.viewMode ?? "list"] ||
      "list") as ViewModeType,
  };
  users.update(data, ["viewMode"]).catch($showError);
  authStore.updateUser(data);
};

const toggleSelection = () => {
  layoutStore.closeHovers();
  if (fileStore.multiple) fileStore.selected = [];
  fileStore.toggleMultiple();
};

// Another folder or page ends the selection mode.
watch(
  () => route.path,
  () => (fileStore.multiple = false)
);
onBeforeUnmount(() => {
  fileStore.multiple = false;
});
</script>
