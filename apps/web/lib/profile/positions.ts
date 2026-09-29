const roleAliases: Record<string, string[]> = {
  "golang / backend": ["Go Developer", "Backend Engineer"],
  "golang / backend engineer": ["Go Developer", "Backend Engineer"],
  "go / backend": ["Go Developer", "Backend Engineer"],
  backend: ["Backend Engineer"],
  "frontend / react": ["Frontend Engineer", "React Developer"],
  "frontend / react engineer": ["Frontend Engineer", "React Developer"],
  "frontend / full stack engineer": [
    "Frontend Engineer",
    "Full Stack Engineer",
  ],
  "full stack": ["Full Stack Engineer"],
  "full-stack engineer": ["Full Stack Engineer"],
  "technical lead": ["Engineering Lead"],
};

export function normalizePositions(positions: string[]) {
  return positions
    .flatMap((position) => {
      const trimmed = position.trim();
      return roleAliases[trimmed.toLocaleLowerCase()] ?? [trimmed];
    })
    .filter(Boolean)
    .filter(
      (position, index, values) =>
        values.findIndex(
          (value) => value.toLocaleLowerCase() === position.toLocaleLowerCase(),
        ) === index,
    );
}
