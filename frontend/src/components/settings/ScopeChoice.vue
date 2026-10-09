<template>
  <div
    class="scope-choice"
    role="radiogroup"
    :aria-label="t('settings.access.title')"
  >
    <label class="option">
      <input
        type="radio"
        :name="name"
        :checked="mode === 'own'"
        @change="emit('update:mode', 'own')"
      />
      <span class="option-text">
        {{ t("settings.access.own") }}
        <span class="option-path">
          {{ t("settings.access.ownHelp", { path: ownPath }) }}
        </span>
      </span>
    </label>
    <label class="option">
      <input
        type="radio"
        :name="name"
        :checked="mode === 'all'"
        @change="emit('update:mode', 'all')"
      />
      <span class="option-text">{{ t("settings.access.all") }}</span>
    </label>
    <label class="option">
      <input
        type="radio"
        :name="name"
        :checked="mode === 'folder'"
        @change="emit('update:mode', 'folder')"
      />
      <span class="option-text">
        {{ t("settings.access.folder") }}
        <span class="option-path">
          {{ folder || t("settings.access.folderHelp") }}
        </span>
      </span>
      <button type="button" class="button button--flat" @click.prevent="pick">
        <icon name="folder_open" />{{ t("buttons.select") }}
      </button>
    </label>
  </div>
</template>

<script setup lang="ts">
import { files, users } from "@/api";
import Icon from "@/components/Icon.vue";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { folderOfURL, type ScopeMode } from "@/utils/scope";
import { inject } from "vue";
import { useI18n } from "vue-i18n";

// The access of a user, or of new users (Gezgin, K148): their own folder, every file, or a folder
// picked in the tree, which opens at the top of the admin's own files.
defineProps<{
  mode: ScopeMode;
  folder: string;
  ownPath: string;
  name: string;
}>();

const emit = defineEmits<{
  (e: "update:mode", value: ScopeMode): void;
  (e: "update:folder", value: string): void;
}>();

const { t } = useI18n();
const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;

const pick = async () => {
  try {
    const [start, admin] = await Promise.all([
      files.fetch("/files/"),
      users.get(authStore.user!.id),
    ]);
    layoutStore.showHover({
      prompt: "folder-picker",
      props: { start },
      confirm: (url: string) => {
        emit("update:folder", folderOfURL(url, admin.scope));
        emit("update:mode", "folder");
      },
    });
  } catch (e: any) {
    $showError(e);
  }
};
</script>
