<template>
  <div class="card floating" id="extract">
    <div class="card-title">
      <h2>{{ t("prompts.extract") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ t("prompts.extractMessage") }}</p>
      <ul class="extract-names">
        <li v-for="name in names" :key="name">
          <code>{{ name }}</code>
        </li>
      </ul>
      <p>
        <label for="extract-password">{{ t("prompts.extractPassword") }}</label>
        <input
          id="extract-password"
          class="input input--block"
          type="password"
          autocomplete="off"
          v-model="password"
          @keyup.enter="submit"
        />
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
        :disabled="busy || items.length === 0"
        :aria-label="t('buttons.extract')"
        :title="t('buttons.extract')"
      >
        {{ t("buttons.extract") }}
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

// Extract starts a job that opens the selected archives on the server (Gezgin).

const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const archiveStore = useArchiveStore();
const $showError = inject<IToastError>("$showError")!;

const password = ref<string>("");
const busy = ref<boolean>(false);

const items = computed(() =>
  fileStore.selected
    .map((i) => fileStore.req?.items[i])
    .filter((item): item is ResourceItem => item !== undefined)
);
const names = computed(() => items.value.map((item) => item.name));

const submit = async () => {
  if (busy.value || items.value.length === 0) return;
  busy.value = true;
  try {
    await archiveStore.start(
      items.value.map((item) => item.path),
      password.value
    );
    layoutStore.closeHovers();
  } catch (e: any) {
    $showError(e);
  } finally {
    busy.value = false;
  }
};
</script>
