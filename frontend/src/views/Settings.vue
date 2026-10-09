<template>
  <div>
    <header-bar showMenu showLogo showNav />

    <div class="settings">
      <!-- Four tabs for an admin, none for a user, who has only their account (Gezgin, K150). -->
      <div
        v-if="user?.perm.admin"
        class="settings-tabs"
        role="navigation"
        :aria-label="t('sidebar.settings')"
      >
        <router-link
          to="/settings/profile"
          :class="{ active: route.path === '/settings/profile' }"
        >
          <icon name="person" />{{ t("settings.tabs.account") }}
        </router-link>
        <router-link
          to="/settings/global"
          :class="{ active: route.path === '/settings/global' }"
        >
          <icon name="tune" />{{ t("settings.tabs.general") }}
        </router-link>
        <router-link
          to="/settings/users"
          :class="{ active: route.path.startsWith('/settings/users') }"
        >
          <icon name="group" />{{ t("settings.tabs.users") }}
        </router-link>
        <router-link
          to="/settings/server"
          :class="{ active: route.path === '/settings/server' }"
        >
          <icon name="dns" />{{ t("settings.tabs.server") }}
        </router-link>
      </div>

      <h2 v-if="loading" class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>

      <router-view></router-view>
    </div>
  </div>
</template>

<script setup lang="ts">
import HeaderBar from "@/components/header/HeaderBar.vue";
import Icon from "@/components/Icon.vue";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";

const { t } = useI18n();
const route = useRoute();

const authStore = useAuthStore();
const layoutStore = useLayoutStore();

const user = computed(() => authStore.user);
const loading = computed(() => layoutStore.loading);
</script>
