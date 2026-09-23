# HireRadar Agent Instructions

## Git workflow

- After completing and verifying a logical change, provide one suggested commit message.
- Use Conventional Commits for suggested messages, for example:
  - `feat: add user profile`
  - `fix: resolve telegram linking`
  - `chore: configure monorepo`
  - `docs: document system architecture`
- Keep each suggested commit message scoped to the completed change.
- Do not include unrelated user changes in a suggested commit.

## Working agreement

- Document accepted architectural decisions and planned work in the repository.
- Keep documentation synchronized when implementation decisions change.
- Verify changes with the relevant lint, type-check, test, and build commands.
- Report which checks passed and note checks that could not be run.
