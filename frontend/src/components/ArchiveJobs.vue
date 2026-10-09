<template>
  <div
    v-if="archiveStore.visible.length > 0"
    class="archive-jobs"
    :class="{ 'above-uploads': uploadStore.activeUploads.size > 0 }"
  >
    <div class="card floating">
      <div class="card-title">
        <h2>{{ t("archive.title") }}</h2>
        <button
          v-if="!running"
          class="action"
          @click="archiveStore.dismiss"
          :aria-label="t('buttons.close')"
          :title="t('buttons.close')"
        >
          <i class="material-icons">close</i>
        </button>
      </div>

      <div class="card-content">
        <div
          v-for="job in archiveStore.visible"
          :key="job.id"
          class="archive-job"
          :data-state="job.state"
        >
          <div class="archive-job__name">
            <i class="material-icons">{{ icon(job) }}</i>
            <template v-if="job.kind === 'create'">
              <span>{{ job.name }}</span>
            </template>
            <template v-else>
              <span>{{ job.names[0] }}</span>
              <span v-if="job.names.length > 1">
                +{{ job.names.length - 1 }}
              </span>
            </template>
          </div>
          <div class="archive-job__status">
            <template v-if="job.state === 'running'">
              <span>{{ progress(job) }}</span>
              <button
                class="button button--flat button--red"
                @click="cancel(job.id)"
              >
                {{ t("buttons.cancel") }}
              </button>
            </template>
            <template v-else-if="job.state === 'done' && job.kind === 'create'">
              <span>{{
                (job.volumes ?? 1) > 1
                  ? t("archive.createdParts", {
                      name: base(job.result).replace(/\.001$/, ""),
                      count: job.volumes,
                    })
                  : t("archive.created", { name: base(job.result) })
              }}</span>
              <router-link class="link" :to="folderLink(job.folder)">
                {{ t("archive.open") }}
              </router-link>
              <span v-if="job.skipped" class="archive-job__note">
                {{ t("archive.skipped", { count: job.skipped }) }}
              </span>
              <span v-if="job.windows" class="archive-job__note">
                {{ t("archive.windows", { count: job.windows }) }}
              </span>
            </template>
            <template v-else-if="job.state === 'done'">
              <span>{{ t("archive.done", { name: base(job.result) }) }}</span>
              <router-link class="link" :to="folderLink(job.result)">
                {{ t("archive.open") }}
              </router-link>
            </template>
            <span v-else-if="job.state === 'cancelled'">
              {{ t("archive.cancelled") }}
            </span>
            <span v-else class="archive-job__error">{{ reason(job) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useArchiveStore } from "@/stores/archive";
import { useFileStore } from "@/stores/file";
import { useUploadStore } from "@/stores/upload";
import { filesize } from "@/utils";
import { encodePath } from "@/utils/url";
import type { ArchiveJob } from "@/api/archive";

// ArchiveJobs shows the archive jobs (Gezgin) and refreshes the listing their folder lands in.

const { t, te } = useI18n();
const archiveStore = useArchiveStore();
const fileStore = useFileStore();
const uploadStore = useUploadStore();
const $showError = inject<IToastError>("$showError")!;

const icons: Record<ArchiveJob["state"], string> = {
  running: "unarchive",
  done: "check_circle",
  failed: "error",
  cancelled: "cancel",
};

const icon = (job: ArchiveJob) =>
  job.state === "running" && job.kind === "create"
    ? "archive"
    : icons[job.state];

const progress = (job: ArchiveJob) => {
  if (job.kind !== "create") {
    return t("archive.running", {
      size: filesize(job.bytes),
      count: job.entries,
    });
  }
  if (!job.planned) {
    return t("archive.planning", { count: job.files ?? 0 });
  }
  const total = job.total ?? 0;
  return t("archive.creating", {
    percent: `%${total > 0 ? Math.floor((job.bytes * 100) / total) : 100}`,
    size: filesize(job.bytes),
    total: filesize(total),
  });
};

const running = computed(() =>
  archiveStore.visible.some((job) => job.state === "running")
);

const trim = (p: string) => p.replace(/\/+$/, "") || "/";
const base = (p?: string) => (p ?? "").split("/").pop() ?? "";
const folderLink = (p?: string) => `/files${encodePath(p ?? "/")}/`;

const reason = (job: ArchiveJob) => {
  const keys = [`archive.errors.${job.error}`, "archive.errors.internal"];
  if (job.kind === "create") {
    keys.unshift(`archive.createErrors.${job.error}`);
  }
  return t(keys.find((key) => te(key)) ?? "archive.errors.internal");
};

const cancel = async (id: string) => {
  try {
    await archiveStore.cancel(id);
  } catch (e: any) {
    $showError(e);
  }
};

// A job that ended in the folder on the screen shows there.
const seen = new Map<string, ArchiveJob["state"]>();
watch(
  () => archiveStore.jobs,
  (jobs) => {
    const here = fileStore.req?.isDir ? trim(fileStore.req.path) : null;
    let reload = false;
    for (const job of jobs) {
      if (
        seen.get(job.id) === "running" &&
        job.state === "done" &&
        trim(job.folder) === here
      ) {
        reload = true;
      }
      seen.set(job.id, job.state);
    }
    if (reload) {
      fileStore.reload = true;
    }
  }
);

onMounted(() => {
  archiveStore.refresh().catch(() => {});
});
</script>
