import "server-only";

import type { components } from "@/lib/api/generated";
import { apiRequest, authenticatedRequest } from "@/lib/api/server";

type APIJob = components["schemas"]["Job"];
type Match = components["schemas"]["Result"];
type SavedJob = components["schemas"]["SavedJob"];

export type Job = {
  id: string;
  title: string;
  company: string;
  location: string;
  employment: string;
  salary: string;
  posted: string;
  match: number;
  summary: string;
  description: string;
  skills: string[];
  missingSkills: string[];
  source: string;
  applyUrl: string;
  remotePolicy: string;
  eligibility: string;
};

function formatSalary(salary?: components["schemas"]["SalaryRange"]) {
  if (!salary) return "—";
  const range = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: salary.currency,
    maximumFractionDigits: 0,
  });
  return `${range.format(salary.minimum)}–${range.format(salary.maximum)} / ${salary.period}`;
}

function formatPosted(value?: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(value));
}

function summarize(description: string) {
  const plain = description
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  return plain.length > 220 ? `${plain.slice(0, 217)}…` : plain;
}

function mapJob(job: APIJob, match?: Match): Job {
  return {
    id: job.id,
    title: job.title,
    company: job.company,
    location: job.location || job.remote_policy,
    employment: (job.employment_types ?? []).join(" · ") || "—",
    salary: formatSalary(job.salary),
    posted: formatPosted(job.published_at ?? job.first_seen_at),
    match: match?.score ?? 0,
    summary: summarize(job.description),
    description: job.description,
    skills: (job.skills ?? []).map(({ name }) => name),
    missingSkills: match?.exclusions ?? [],
    source: `Priority ${job.source_priority}`,
    applyUrl: job.apply_url,
    remotePolicy: job.remote_policy,
    eligibility: job.eligibility,
  };
}

async function getMatchMap() {
  const response = await authenticatedRequest("/api/v1/matches").catch(
    () => null,
  );
  if (!response?.ok) return new Map<string, Match>();
  const body = (await response.json()) as {
    matches: Match[] | null;
  };
  return new Map((body.matches ?? []).map((match) => [match.job_id, match]));
}

export async function getJobs(query = "status=active&limit=100") {
  const [response, matches] = await Promise.all([
    apiRequest(`/api/v1/jobs?${query}`).catch(() => null),
    getMatchMap(),
  ]);
  if (!response?.ok) return [];
  const body = (await response.json()) as components["schemas"]["ListResult"];
  return (body.items ?? []).map((job) => mapJob(job, matches.get(job.id)));
}

export async function getJob(id: string) {
  const [response, matches] = await Promise.all([
    apiRequest(`/api/v1/jobs/${encodeURIComponent(id)}`).catch(() => null),
    getMatchMap(),
  ]);
  if (!response?.ok) return null;
  const job = (await response.json()) as APIJob;
  return mapJob(job, matches.get(job.id));
}

export async function getSavedJobs() {
  const response = await authenticatedRequest(
    "/api/v1/saved-jobs?limit=100",
  ).catch(() => null);
  if (!response?.ok) return [];
  const body = (await response.json()) as {
    items: SavedJob[] | null;
  };
  return (body.items ?? []).map((job): Job => ({
    id: job.job_id,
    title: job.title,
    company: job.company,
    location: job.location,
    employment: job.status,
    salary: "—",
    posted: formatPosted(job.saved_at),
    match: 0,
    summary: "",
    description: "",
    skills: [],
    missingSkills: [],
    source: "",
    applyUrl: job.apply_url,
    remotePolicy: "",
    eligibility: "",
  }));
}
