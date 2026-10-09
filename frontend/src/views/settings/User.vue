<template>
  <errors v-if="error" :errorCode="error.status" />
  <form v-else-if="!layoutStore.loading && user" @submit.prevent="save">
    <router-link to="/settings/users" class="back-link">
      <icon name="arrow_back" />{{ t("settings.users") }}
    </router-link>

    <div class="settings-card">
      <h3>{{ isNew ? t("settings.newUser") : t("settings.identity") }}</h3>
      <setting-row :label="t('settings.username')" label-for="username" stack>
        <input
          id="username"
          class="input"
          type="text"
          v-model="user.username"
          autocomplete="off"
          required
        />
      </setting-row>
      <setting-row
        :label="isNew ? t('settings.password') : t('settings.newPassword')"
        label-for="password"
        stack
      >
        <input
          id="password"
          class="input"
          type="password"
          v-model="user.password"
          :placeholder="isNew ? '' : t('settings.passwordKeep')"
          autocomplete="new-password"
          :required="isNew"
        />
      </setting-row>
      <template v-if="!isSelf">
        <setting-row
          :label="t('settings.mustChangePassword')"
          :help="t('settings.mustChangePasswordHelp')"
        >
          <toggle-switch
            :model-value="user.mustChangePassword ?? false"
            :label="t('settings.mustChangePassword')"
            @update:model-value="setMustChange"
          />
        </setting-row>
        <setting-row
          :label="t('settings.lockPassword')"
          :help="user.perm.admin ? t('settings.lockPasswordAdmin') : undefined"
        >
          <toggle-switch
            v-model="user.lockPassword"
            :label="t('settings.lockPassword')"
            :disabled="user.perm.admin"
          />
        </setting-row>
      </template>
      <setting-row :label="t('settings.language')" label-for="locale">
        <languages id="locale" class="input" v-model:locale="user.locale" />
      </setting-row>
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.access.title") }}</h3>
      <p class="setting-help">{{ t("settings.access.help") }}</p>
      <scope-choice
        v-model:mode="access.mode"
        v-model:folder="access.folder"
        :own-path="ownPath"
        name="scope"
      />
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.permissions") }}</h3>
      <permissions v-model:perm="user.perm" :is-default="false" />
    </div>

    <div class="settings-card">
      <details :open="user.rules.length > 0">
        <summary>
          <icon name="chevron_right" />{{ t("settings.advancedUser") }}
        </summary>
        <p class="setting-help">{{ t("settings.rulesHelp") }}</p>
        <rules v-model:rules="user.rules" />
      </details>
    </div>

    <div class="settings-footer">
      <div>
        <template v-if="!isNew">
          <button
            type="button"
            class="button button--flat"
            @click="closeSessions"
          >
            {{ t("settings.closeSessions") }}
          </button>
          <button
            type="button"
            class="button button--flat button--red"
            @click="deletePrompt"
          >
            {{ t("settings.deleteUser") }}
          </button>
        </template>
      </div>
      <div>
        <router-link
          to="/settings/users"
          class="button button--flat button--grey"
        >
          {{ t("buttons.cancel") }}
        </router-link>
        <button type="submit" class="button">{{ t("buttons.save") }}</button>
      </div>
    </div>
  </form>
</template>

<script setup lang="ts">
import { settings, users as api } from "@/api";
import { StatusError } from "@/api/utils";
import Icon from "@/components/Icon.vue";
import Languages from "@/components/settings/Languages.vue";
import Permissions from "@/components/settings/Permissions.vue";
import Rules from "@/components/settings/Rules.vue";
import ScopeChoice from "@/components/settings/ScopeChoice.vue";
import SettingRow from "@/components/settings/SettingRow.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { locales } from "@/i18n";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { login, logout } from "@/utils/auth";
import { homeDir, scopeMode, type ScopeMode } from "@/utils/scope";
import Errors from "@/views/Errors.vue";
import { computed, inject, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// A user's page (Gezgin, K145, K148, K154): who they are and their password, which may be asked
// to change at the next login; their access; their permissions; folded, their rules.
const error = ref<StatusError | null>(null);
const user = ref<IUser | null>(null);
const homesBase = ref<string>("/users");
const access = ref<{ mode: ScopeMode; folder: string }>({
  mode: "all",
  folder: "",
});
// Once the admin touched the forced change, a new password no longer turns it on.
const mustChangeTouched = ref<boolean>(false);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const isNew = computed(() => route.path === "/settings/users/new");
const isSelf = computed(
  () => !isNew.value && user.value?.id === authStore.user?.id
);
const ownPath = computed(() =>
  homeDir(homesBase.value, user.value?.username || t("settings.access.name"))
);

const fetchData = async () => {
  layoutStore.loading = true;
  mustChangeTouched.value = false;

  try {
    const set = await settings.get();
    homesBase.value = set.userHomeBasePath;
    if (isNew.value) {
      user.value = {
        ...(set.defaults as unknown as IUser),
        username: "",
        password: "",
        rules: [],
        lockPassword: false,
        // The password the admin types is a temporary one (K145).
        mustChangePassword: true,
        id: 0,
      };
      access.value = set.createUserDir
        ? { mode: "own", folder: "" }
        : scopeMode(set.defaults.scope, null);
    } else {
      const id = Array.isArray(route.params.id)
        ? route.params.id.join("")
        : route.params.id;
      const stored = await api.get(parseInt(id));
      user.value = { ...stored, password: "", rules: stored.rules ?? [] };
      access.value = scopeMode(
        stored.scope,
        homeDir(set.userHomeBasePath, stored.username)
      );
    }
    // A language Gezgin no longer has reads as English (K142).
    if (!locales.includes(user.value.locale)) user.value.locale = "en";
  } catch (err) {
    if (err instanceof Error) {
      error.value = err as StatusError;
    }
  } finally {
    layoutStore.loading = false;
  }
};

onMounted(fetchData);
watch(() => route.path, fetchData);

const setMustChange = (value: boolean) => {
  mustChangeTouched.value = true;
  if (user.value) user.value.mustChangePassword = value;
};

// A password the admin sets for someone is a temporary one, unless they say otherwise.
watch(
  () => user.value?.password,
  (password) => {
    if (!user.value || isSelf.value || mustChangeTouched.value) return;
    if (password) user.value.mustChangePassword = true;
  }
);

// An admin can always change their password.
watch(
  () => user.value?.perm.admin,
  (admin) => {
    if (admin && user.value) user.value.lockPassword = false;
  }
);

const askPassword = (action: (currentPassword: string) => void) => {
  layoutStore.showHover({
    prompt: "current-password",
    confirm: (event: Event, currentPassword: string) => {
      event.preventDefault();
      layoutStore.closeHovers();
      action(currentPassword);
    },
  });
};

const deletePrompt = () => askPassword(deleteUser);

const deleteUser = async (currentPassword: string) => {
  if (!user.value) return;
  try {
    await api.remove(user.value.id, currentPassword);
    if (user.value.id == authStore.user?.id) {
      logout();
    } else {
      router.push({ path: "/settings/users" });
    }
    $showSuccess(t("settings.userDeleted"));
  } catch (err) {
    if (err instanceof StatusError) {
      err.status === 403 ? $showError(t("errors.forbidden")) : $showError(err);
    } else if (err instanceof Error) {
      $showError(err);
    }
  }
};

const closeSessions = async () => {
  if (!user.value) return;

  try {
    await api.closeSessions(user.value.id);
    if (user.value.id === authStore.user?.id) {
      logout("sessions");
    } else {
      $showSuccess(t("settings.sessionsClosed"));
    }
  } catch (e: any) {
    $showError(e);
  }
};

const save = () => {
  if (!user.value) return;
  if (access.value.mode === "folder" && access.value.folder === "") {
    $showError(t("settings.access.noFolder"));
    return;
  }
  askPassword(send);
};

const send = async (currentPassword: string) => {
  if (!user.value) return;

  const own = access.value.mode === "own";
  const data: IUser = {
    ...user.value,
    scope:
      access.value.mode === "folder"
        ? access.value.folder
        : own
          ? ownPath.value
          : ".",
  };
  if (isSelf.value) data.mustChangePassword = false;

  try {
    if (isNew.value) {
      await api.create(data, currentPassword, own);
      router.push({ path: "/settings/users" });
      $showSuccess(t("settings.userCreated"));
    } else {
      await api.update(data, ["all"], currentPassword, own);

      if (data.id === authStore.user?.id) {
        if (data.password) {
          // A new password ended the admin's own sessions; it opens the next one.
          await login(data.username, data.password);
        } else {
          authStore.updateUser(data);
        }
      }

      user.value.password = "";
      mustChangeTouched.value = false;
      $showSuccess(t("settings.userUpdated"));
    }
  } catch (e: any) {
    if (e instanceof StatusError && e.status === 409) {
      $showError(t("login.usernameTaken"));
    } else if (e instanceof StatusError && e.message.includes("sole admin")) {
      $showError(t("settings.lastAdmin"));
    } else {
      $showError(e);
    }
  }
};
</script>
