<template>
  <div>
    <h3>{{ $t("settings.permissions") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>

    <!-- The defaults never grant the admin permission (Gezgin). -->
    <p v-if="!isDefault">
      <input type="checkbox" v-model="admin" />
      {{ $t("settings.administrator") }}
    </p>

    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.create" />
      {{ $t("settings.perm.create") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.delete" />
      {{ $t("settings.perm.delete") }}
    </p>
    <p>
      <input
        type="checkbox"
        :disabled="admin || perm.share"
        v-model="perm.download"
      />
      {{ $t("settings.perm.download") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.modify" />
      {{ $t("settings.perm.modify") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.rename" />
      {{ $t("settings.perm.rename") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.share" />
      {{ $t("settings.perm.share") }}
    </p>
  </div>
</template>

<script>
export default {
  name: "permissions",
  props: ["perm", "isDefault"],
  computed: {
    admin: {
      get() {
        return this.perm.admin;
      },
      set(value) {
        if (value) {
          for (const key in this.perm) {
            this.perm[key] = true;
          }
        }

        this.perm.admin = value;
      },
    },
  },
  watch: {
    perm: {
      deep: true,
      handler() {
        if (this.perm.share === true) {
          this.perm.download = true;
        }
      },
    },
  },
};
</script>
