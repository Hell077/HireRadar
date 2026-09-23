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
  skills: string[];
  missingSkills: string[];
  source: string;
};

export const jobs: Job[] = [
  {
    id: "senior-go-engineer-northstar",
    title: "Senior Go Engineer",
    company: "Northstar Labs",
    location: "Worldwide",
    employment: "Contract · Full-time",
    salary: "$6,000–8,000 / month",
    posted: "2h ago",
    match: 94,
    summary:
      "Build reliable distributed services for a remote-first developer platform used by engineering teams worldwide.",
    skills: ["Go", "PostgreSQL", "Docker", "REST APIs"],
    missingSkills: ["Kubernetes"],
    source: "Ashby",
  },
  {
    id: "frontend-engineer-arcade",
    title: "Frontend Engineer — React",
    company: "Arcade Systems",
    location: "EMEA",
    employment: "B2B · Full-time",
    salary: "€70,000–90,000 / year",
    posted: "5h ago",
    match: 89,
    summary:
      "Own customer-facing product experiences with React, Next.js and TypeScript in a small distributed product team.",
    skills: ["React", "Next.js", "TypeScript", "Tailwind CSS"],
    missingSkills: ["Playwright"],
    source: "Greenhouse",
  },
  {
    id: "fullstack-go-react-fluent",
    title: "Full-stack Engineer (Go + React)",
    company: "Fluent Works",
    location: "Worldwide",
    employment: "Contract · Part-time",
    salary: "$65–85 / hour",
    posted: "Yesterday",
    match: 86,
    summary:
      "Ship end-to-end workflow features across Go services and a modern React application with an async-first team.",
    skills: ["Go", "React", "TypeScript", "PostgreSQL"],
    missingSkills: ["GraphQL"],
    source: "Lever",
  },
];

export function getJob(id: string) {
  return jobs.find((job) => job.id === id);
}
