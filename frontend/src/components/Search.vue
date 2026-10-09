<template>
  <form id="search" role="search" :class="{ active }" @submit.prevent="submit">
    <div id="input">
      <button
        v-if="active"
        type="button"
        class="action"
        @click="close"
        :aria-label="t('buttons.close')"
        :title="t('buttons.close')"
      >
        <icon name="arrow_back" />
      </button>
      <icon v-else name="search" />
      <input
        ref="input"
        type="text"
        autocomplete="off"
        enterkeyhint="search"
        v-model="prompt"
        @keydown.esc.prevent="clear"
        :aria-label="t('search.search')"
        :placeholder="t('search.search')"
      />
      <button
        v-if="prompt !== ''"
        type="button"
        class="action clear"
        @click="clear"
        :aria-label="t('buttons.clear')"
        :title="t('buttons.clear')"
      >
        <icon name="close" />
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import { useLayoutStore } from "@/stores/layout";
import { storeToRefs } from "pinia";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// The search bar (Gezgin, K110): typed in where it is; Enter searches the open folder and shows
// the results as a folder view of their own (K111), Esc or × empties it. On a phone the
// magnifier opens it over the header (the "search" hover). Conditions such as type:image or
// case:sensitive go in the query.

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const layoutStore = useLayoutStore();
const { currentPromptName } = storeToRefs(layoutStore);

const input = ref<HTMLInputElement | null>(null);
const active = computed(() => currentPromptName.value === "search");

// On the results page the bar keeps the query.
const queryOf = () =>
  route.name === "Search" ? String(route.query.q ?? "") : "";
const prompt = ref<string>(queryOf());
watch(
  () => route.fullPath,
  () => (prompt.value = queryOf())
);

watch(active, async (open) => {
  if (!open) return;
  await nextTick();
  input.value?.focus();
});

// The folder searched: the one open, or the one the results are of.
const base = () => {
  const prefix = route.name === "Search" ? "/search" : "/files";
  const p = route.path.slice(prefix.length) || "/";
  return p.endsWith("/") ? p : p + "/";
};

const submit = () => {
  const q = prompt.value.trim();
  if (q === "") return;
  if (active.value) layoutStore.closeHovers();
  input.value?.blur();
  router.push({ path: "/search" + base(), query: { q } });
};

const clear = () => {
  if (prompt.value === "" && active.value) {
    close();
    return;
  }
  prompt.value = "";
  input.value?.focus();
};

const close = () => {
  layoutStore.closeHovers();
};
</script>
