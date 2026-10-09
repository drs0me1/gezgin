<template>
  <div
    class="segmented"
    role="radiogroup"
    :aria-label="t('settings.themes.title')"
  >
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      :aria-checked="theme === option.value"
      :class="{ active: theme === option.value }"
      @click="emit('update:theme', option.value)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

// The interface's theme as three buttons (Gezgin, K151): the system's, light or dark.
defineProps<{
  theme: UserTheme;
}>();

const emit = defineEmits<{
  (e: "update:theme", value: UserTheme): void;
}>();

const { t } = useI18n();

const options = computed<{ value: UserTheme; label: string }[]>(() => [
  { value: "", label: t("settings.themes.default") },
  { value: "light", label: t("settings.themes.light") },
  { value: "dark", label: t("settings.themes.dark") },
]);
</script>
