<template>
  <div class="card floating">
    <div class="card-content">
      <p>{{ props.message }}</p>
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
        :class="props.danger ? 'button--red' : 'button--blue'"
        @click="layoutStore.currentPrompt?.confirm()"
        :aria-label="props.confirm"
        :title="props.confirm"
      >
        {{ props.confirm }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { useI18n } from "vue-i18n";

// A question before an action on many items (Gezgin): its message, the action's name, and red
// when it cannot be undone.
const { t } = useI18n();
const layoutStore = useLayoutStore();
const props: { message?: string; confirm?: string; danger?: boolean } =
  layoutStore.currentPrompt?.props ?? {};
</script>
