import { defineStore } from "pinia";
import { archive as api } from "@/api";
import type { ArchiveJob } from "@/api/archive";

// The archive jobs (Gezgin): the user's latest, polled while one runs. Those started or seen
// running on this page are shown until dismissed.
export const useArchiveStore = defineStore("archive", {
  state: (): {
    jobs: ArchiveJob[];
    shown: string[];
    timer: number | null;
  } => ({
    jobs: [],
    shown: [],
    timer: null,
  }),
  getters: {
    visible: (state) =>
      state.jobs.filter((job) => state.shown.includes(job.id)),
  },
  actions: {
    async start(paths: string[], password: string) {
      const job = await api.extract(paths, password);
      this.jobs = [job, ...this.jobs.filter((j) => j.id !== job.id)];
      this.shown.push(job.id);
      this.poll();
    },
    async refresh() {
      this.jobs = await api.list();
      for (const job of this.jobs) {
        if (job.state === "running" && !this.shown.includes(job.id)) {
          this.shown.push(job.id);
        }
      }
      if (this.jobs.some((job) => job.state === "running")) {
        this.poll();
      }
    },
    poll() {
      if (this.timer !== null) return;
      this.timer = window.setTimeout(async () => {
        this.timer = null;
        try {
          await this.refresh();
        } catch {
          // A failed look is tried again; the job goes on on the server.
          this.poll();
        }
      }, 1000);
    },
    async cancel(id: string) {
      await api.cancel(id);
      await this.refresh();
    },
    // dismiss hides the jobs that ended.
    dismiss() {
      this.shown = this.jobs
        .filter((job) => job.state === "running" && this.shown.includes(job.id))
        .map((job) => job.id);
    },
  },
});
