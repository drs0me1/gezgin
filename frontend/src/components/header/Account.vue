<template>
  <div class="account" ref="root">
    <button
      class="action account-button"
      @click="open = !open"
      :aria-label="t('sidebar.account', { name: user?.username })"
      :title="t('sidebar.account', { name: user?.username })"
      aria-haspopup="menu"
      :aria-expanded="open"
    >
      <icon name="account_circle" />
      <span class="account-name">{{ user?.username }}</span>
    </button>
    <div v-if="open" class="account-menu" role="menu">
      <button class="action" role="menuitem" @click="go('/settings/profile')">
        <icon name="person" />
        <span>{{ t("settings.profileSettings") }}</span>
      </button>
      <button
        v-if="user?.perm.admin"
        class="action"
        role="menuitem"
        @click="go('/settings/global')"
      >
        <icon name="settings" />
        <span>{{ t("sidebar.settings") }}</span>
      </button>
      <button class="action" role="menuitem" @click="logout">
        <icon name="logout" />
        <span>{{ t("sidebar.logout") }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import * as auth from "@/utils/auth";
import { storeToRefs } from "pinia";
import { onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// The account at the header's right (Gezgin, K120): the user's name, opening their profile
// settings, an admin's settings and the logout.

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const layoutStore = useLayoutStore();
const { user } = storeToRefs(useAuthStore());

const open = ref<boolean>(false);
const root = ref<HTMLElement | null>(null);

const go = (path: string) => {
  open.value = false;
  layoutStore.closeHovers();
  router.push(path);
};

const logout = () => {
  open.value = false;
  auth.logout();
};

// A click elsewhere or Esc closes the menu.
const outside = (event: MouseEvent) => {
  if (!root.value?.contains(event.target as Node)) open.value = false;
};
const escape = (event: KeyboardEvent) => {
  if (event.key === "Escape") open.value = false;
};
watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener("click", outside);
    document.addEventListener("keydown", escape);
  } else {
    document.removeEventListener("click", outside);
    document.removeEventListener("keydown", escape);
  }
});
watch(
  () => route.fullPath,
  () => (open.value = false)
);
onBeforeUnmount(() => {
  document.removeEventListener("click", outside);
  document.removeEventListener("keydown", escape);
});
</script>
