# HireRadar Design Direction

## Product character

HireRadar should feel calm, focused, trustworthy, and practical. The interface supports repeated job-search work, so clarity and low visual fatigue take priority over decorative effects.

The product must not use neon colors, glowing elements, excessive gradients, glassmorphism, oversized shadows, or attention-seeking animation. Visual hierarchy should come from typography, spacing, contrast, and restrained use of the brand color.

## Visual foundation

- Use a neutral background with slightly differentiated surfaces for cards, panels, and navigation.
- Use dark neutral text with sufficient contrast and softer neutral colors for secondary information.
- Select one restrained accent color for primary actions, active navigation, links, and focus states.
- Keep success, warning, and error colors muted but distinguishable and accessible.
- Prefer subtle borders over heavy shadows. Use shadows only when they communicate elevation.
- Use consistent medium-radius corners. Avoid making every container look like a floating card.
- Use a highly readable sans-serif typeface with a compact, professional hierarchy.

All visual values must be represented by semantic design tokens in the shared global stylesheet and consumed through Tailwind utilities.

## Responsive approach

Desktop and mobile are two compositions of the same product, not separate visual systems. They share tokens, typography, components, content priority, and interaction states.

Primary design frames:

- Desktop: 1440 px
- Mobile: 390 px
- Tablet validation: 768 px

Implementation remains fluid between breakpoints; the frames are design references rather than fixed target widths.

### Desktop navigation

- Persistent top header for primary product areas, notifications, and account actions.
- Keep the same navigation placement across desktop product screens.
- Main content uses a readable maximum width instead of stretching across the viewport.
- Job list and job details may use a split view where space permits.

### Mobile navigation

- Bottom navigation for the most frequent destinations.
- Secondary destinations and account actions live in a menu or settings screen.
- Job list and job details become separate views.
- Filters open in a full-height sheet or focused screen.
- Primary actions remain reachable without relying on hover interactions.
- Tap targets must be at least 44 by 44 px.

## Initial screen set

1. Public landing page
2. Sign in and registration
3. Profile onboarding
4. Resume upload and extracted-skill confirmation
5. Job preferences and source selection
6. Telegram connection
7. Personalized job feed
8. Job details and match explanation
9. Saved and dismissed jobs
10. Profile, sources, and notification settings

Each product screen must be designed in desktop and mobile variants before implementation. Empty, loading, error, and success states are part of the screen definition rather than follow-up work.

## Core components

- App shell and navigation
- Button and icon button
- Input, textarea, select, checkbox, and form field
- Card and section container
- Badge and status indicator
- Job card
- Match score and match explanation
- Filter controls
- Dialog, sheet, dropdown, tooltip, and toast
- File upload
- Skeleton and empty state

Components must define their important states: default, hover, focus-visible, active, disabled, loading, selected, and error where applicable.

## Motion

Motion should explain state changes and preserve spatial continuity. Use it for route transitions, sheets, dialogs, expanding filters, list updates, and lightweight feedback.

- Prefer short, subtle transitions.
- Do not animate every element on initial page load.
- Avoid decorative looping animation.
- Respect `prefers-reduced-motion`.
- Use CSS transitions for simple color and opacity changes; use Motion only for coordinated, layout, gesture, or presence animation.

Implementation uses shared Motion reveal and route-transition primitives rather than screen-specific timing logic. Interactive controls use short CSS transitions, while operating-system reduced-motion preferences disable movement globally.

## Pen.dev workflow

- Store design files in the repository so design changes can be reviewed alongside code.
- Create a shared design library for colors, typography, spacing, radii, and reusable components.
- Build desktop and mobile frames from instances of the shared components.
- Name layers and components by product purpose, matching code terminology where practical.
- Keep design variables aligned with CSS token names.
- Review the responsive behavior and interaction states before implementing a screen.
- Treat generated HTML or Tailwind output as a reference, not as production code without review.

## Recommended build order

1. Define semantic color, typography, spacing, radius, and shadow tokens.
2. Build the shared component library.
3. Design the application shell and navigation for desktop and mobile.
4. Design the job feed and job details, which establish the main product language.
5. Design onboarding, authentication, and settings using the established system.
6. Validate responsive behavior and component states.
7. Implement the approved screens with shadcn/ui, Tailwind CSS, and Motion.

## Implementation status

- Complete: semantic Tailwind foundation, shared shadcn/ui setup, responsive profile page.
- Complete: responsive sign-in, registration, password recovery, and password reset screens using a shared authentication shell.
- Complete: profile onboarding, resume upload, extracted-data review, job preferences, and Telegram connection preference.
- Complete: English and Russian localization with `next-intl`; the selected locale is stored in a first-party cookie and does not change route URLs.
- Complete: localized personalized job feed and job details with match explanations.
- Complete: localized saved jobs and settings for vacancy sources, notifications, delivery rules, and Telegram.
- Complete: localized profile, skills, salary expectations, and resume editing flows using shared dialog and sheet components.
- Next: reusable loading, empty, error, and success states, followed by backend integration.
