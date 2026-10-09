<template>
  <div class="header-nav">
    <action
      icon="arrow_back"
      :label="t('nav.back', { key: keys.back })"
      :disabled="!canBack"
      @action="router.back()"
    />
    <action
      class="forward"
      icon="arrow_forward"
      :label="t('nav.forward', { key: keys.forward })"
      :disabled="!canForward"
      @action="router.forward()"
    />
    <action
      icon="arrow_upward"
      :label="t('nav.up', { key: keys.up })"
      :disabled="parent === null"
      @action="up"
    />
    <action
      icon="home"
      :label="t('sidebar.myFiles')"
      :disabled="home"
      @action="router.push('/files/')"
    />
  </div>
</template>

<script setup lang="ts">
import Action from "@/components/header/Action.vue";
import { useLayoutStore } from "@/stores/layout";
import { atHome, isMac, isUpKey, navKeys, parentOf } from "@/utils/navigation";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// The header's back, forward, up and home (Gezgin, K99-K103). Back and forward move through
// Gezgin's own history in the tab, as the browser's buttons do, and are dimmed where that
// history ends, so that they never leave Gezgin; up opens the folder the open one lies in (which
// then shows the folder left selected) and home the top of "Dosyalarım".

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const layoutStore = useLayoutStore();

const mac = isMac(navigator.platform || navigator.userAgent);
const keys = navKeys(mac);

// The router keeps where the tab came from and went back from in its history entries.
const canBack = ref<boolean>(false);
const canForward = ref<boolean>(false);
const readHistory = () => {
  const state = router.options.history.state;
  canBack.value = state.back != null;
  canForward.value = state.forward != null;
};
watch(() => route.fullPath, readHistory, { immediate: true });

const parent = computed(() =>
  route.name === "Files" ? parentOf(route.path) : null
);
const home = computed(() => route.name === "Files" && atHome(route.path));

const up = () => {
  if (parent.value !== null) router.push(parent.value);
};

// Alt+↑, or ⌘↑ on a Mac, opens the parent folder, unless a window is open or a text is typed.
const keyEvent = (event: KeyboardEvent) => {
  if (!isUpKey(event, mac) || layoutStore.currentPrompt !== null) return;
  const target = event.target as HTMLElement | null;
  if (
    target?.isContentEditable ||
    ["INPUT", "TEXTAREA", "SELECT"].includes(target?.tagName ?? "")
  ) {
    return;
  }
  event.preventDefault();
  up();
};

onMounted(() => window.addEventListener("keydown", keyEvent));
onBeforeUnmount(() => window.removeEventListener("keydown", keyEvent));
</script>
