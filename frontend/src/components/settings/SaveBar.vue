<template>
  <div v-if="visible" class="save-bar" role="status">
    <span>{{ t("settings.unsaved") }}</span>
    <span>
      <button
        type="button"
        class="button button--flat button--grey"
        @click="emit('cancel')"
      >
        {{ t("buttons.cancel") }}
      </button>
      <button type="button" class="button" @click="emit('save')">
        {{ t("buttons.save") }}
      </button>
    </span>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

// One save for a settings page (Gezgin, K152): shown once something changed, it stays at the
// bottom of the window.
defineProps<{ visible: boolean }>();

const emit = defineEmits<{
  (e: "cancel"): void;
  (e: "save"): void;
}>();

const { t } = useI18n();
</script>
