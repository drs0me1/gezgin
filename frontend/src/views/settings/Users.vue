<template>
  <errors v-if="error" :errorCode="error.status" />
  <div v-else-if="!layoutStore.loading">
    <div class="settings-toolbar">
      <span class="setting-help">
        {{ t("settings.userCount", users.length) }}
      </span>
      <router-link to="/settings/users/new" class="button">
        <icon name="person_add" />{{ t("settings.newUser") }}
      </router-link>
    </div>

    <div class="settings-card user-list">
      <router-link
        v-for="user in users"
        :key="user.id"
        :to="'/settings/users/' + user.id"
        class="user-row"
      >
        <span class="avatar" aria-hidden="true">{{ initial(user) }}</span>
        <span class="user-main">
          <span class="user-name">
            {{ user.username }}
            <span v-if="user.perm.admin" class="badge admin">
              {{ t("settings.administrator") }}
            </span>
            <span v-if="user.mustChangePassword" class="badge warn">
              {{ t("settings.mustChangeBadge") }}
            </span>
            <span v-if="user.lockPassword" class="badge">
              {{ t("settings.lockedBadge") }}
            </span>
          </span>
          <span class="user-access">{{ access(user) }}</span>
        </span>
        <span class="perm-icons">
          <icon
            v-for="perm in perms"
            :key="perm.key"
            :name="perm.icon"
            :class="{ off: !user.perm[perm.key] }"
            :title="
              (user.perm[perm.key] ? '' : t('settings.permOff') + ': ') +
              t(`settings.permShort.${perm.key}`)
            "
          />
        </span>
        <icon name="chevron_right" />
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { users as api } from "@/api";
import { StatusError } from "@/api/utils";
import Icon from "@/components/Icon.vue";
import { useLayoutStore } from "@/stores/layout";
import { isAll } from "@/utils/scope";
import Errors from "@/views/Errors.vue";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The users (Gezgin, K154): each with their role, access, permissions and the states of their
// password; a click opens the user.
const error = ref<StatusError | null>(null);
const users = ref<IUser[]>([]);

const layoutStore = useLayoutStore();
const { t, locale } = useI18n();

const perms = [
  { key: "create", icon: "create_new_folder" },
  { key: "delete", icon: "delete" },
  { key: "download", icon: "file_download" },
  { key: "modify", icon: "edit_note" },
  { key: "rename", icon: "mode_edit" },
  { key: "share", icon: "share" },
] as const;

const initial = (user: IUser) =>
  user.username.charAt(0).toLocaleUpperCase(locale.value);

const access = (user: IUser) =>
  isAll(user.scope) ? t("settings.access.all") : user.scope;

onMounted(async () => {
  layoutStore.loading = true;

  try {
    users.value = await api.getAll();
  } catch (err) {
    if (err instanceof Error) {
      error.value = err as StatusError;
    }
  } finally {
    layoutStore.loading = false;
  }
});
</script>
