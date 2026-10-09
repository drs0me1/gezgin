<template>
  <div>
    <!-- The defaults never grant the admin permission (Gezgin). -->
    <setting-row
      v-if="!isDefault"
      :label="t('settings.administrator')"
      :help="t('settings.adminHelp')"
    >
      <toggle-switch
        :model-value="perm.admin"
        :label="t('settings.administrator')"
        @update:model-value="setAdmin"
      />
    </setting-row>
    <div class="perm-grid">
      <setting-row
        v-for="key in keys"
        :key="key"
        :label="t(`settings.permShort.${key}`)"
        :help="key === 'share' ? t('settings.shareNeedsDownload') : undefined"
      >
        <toggle-switch
          :model-value="perm[key]"
          :label="t(`settings.permShort.${key}`)"
          :disabled="perm.admin || (key === 'download' && perm.share)"
          @update:model-value="(value: boolean) => set(key, value)"
        />
      </setting-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SettingRow from "./SettingRow.vue";
import ToggleSwitch from "./ToggleSwitch.vue";

// A user's permissions as switches (Gezgin, K154): the admin's at the top, which turns every
// other on, then the six of the files; sharing turns downloading on.
const props = defineProps<{
  perm: Permissions;
  isDefault: boolean;
}>();

const emit = defineEmits<{
  (e: "update:perm", value: Permissions): void;
}>();

const { t } = useI18n();

const keys = [
  "create",
  "delete",
  "download",
  "modify",
  "rename",
  "share",
] as const;

const setAdmin = (admin: boolean) => {
  const perm = { ...props.perm, admin };
  if (admin) for (const key of keys) perm[key] = true;
  emit("update:perm", perm);
};

const set = (key: (typeof keys)[number], value: boolean) => {
  const perm = { ...props.perm, [key]: value };
  if (perm.share) perm.download = true;
  emit("update:perm", perm);
};
</script>
