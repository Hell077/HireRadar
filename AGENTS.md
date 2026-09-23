# HireRadar Agent Instructions

## Git workflow

- After completing and verifying a logical change, provide one suggested commit message.
- Keep each suggested commit message scoped to the completed change.
- Do not include unrelated user changes in a suggested commit.

## Styling and design tokens

- Treat the global stylesheet and Tailwind CSS as the single styling system for the web application.
- Define the visual foundation centrally with semantic CSS variables: backgrounds, foregrounds, surfaces, borders, brand colors, status colors, typography, radii, shadows, and spacing when appropriate.
- Expose those variables through Tailwind utilities and use the semantic utilities in application and shared UI components.
- Do not hardcode reusable colors in JSX, TSX, component-level CSS, or arbitrary Tailwind values. A new reusable visual value must become a design token first.
- Name tokens by purpose rather than by their current appearance. Prefer names such as `background`, `surface`, `primary`, `muted`, and `destructive` instead of names such as `white`, `gray`, or `red`.
- Keep light and dark theme values behind the same semantic token names so components remain theme-independent.
- Shared components must consume the centralized tokens and must not establish a separate color system.
- A visual redesign should primarily require updating global tokens and foundational styles, without rewriting individual components.

## Working agreement

- Document accepted architectural decisions and planned work in the repository.
- Keep documentation synchronized when implementation decisions change.
- Verify changes with the relevant lint, type-check, test, and build commands.
- Report which checks passed and note checks that could not be run.
