# Frontend Guidelines

The Web application uses React and TypeScript. These rules apply to all frontend implementation and review work.

## Naming and structure

- Use function components and Hooks.
- Use kebab-case filenames: `user-profile.tsx`, `data-table.tsx`, and `use-user-data.ts`.
- Use PascalCase component identifiers and camelCase Hook identifiers: `UserProfile` and `useUserData`.
- Define every component's props with an explicit TypeScript interface.
- Do not use `any` in components. Use a specific type, or `unknown` with a type guard.
- Keep components focused on rendering and interaction. Move complex stateful logic into Hooks and pure logic into utilities.
- Prefer the smallest component split that creates a clear responsibility, reusable behavior, or independently testable unit. Do not split markup solely to reduce line count.

## Data and state

- Components must not define API clients or call `fetch` or `axios` directly.
- Wrap every server read or write in a TanStack Query `useQuery` or `useMutation` Hook.
- Place Hooks in feature folders, for example `src/hooks/login/use-login.ts`; do not place standalone Hook files directly under `src/hooks`.
- TanStack Query owns server state and caching.
- React Hook Form owns form state, with Zod schemas for validation.
- `useState` may own component-local, transient UI state.
- Zustand owns shared client state that crosses component or route boundaries. Do not duplicate TanStack Query or React Hook Form state in Zustand.
- Do not introduce React Context or Redux for application state.

## UI and styling

- Use Tailwind CSS for styling. Do not add CSS Modules or styled-components.
- Use [Watermelon UI](https://ui.watermelon.sh/components) as the primary component and block registry. Components install through the shadcn CLI from `https://registry.watermelon.sh/r/<name>.json`; the numbered variant manifests (e.g. `button-1.json`) pull the full Watermelon implementation for a component, so verify the installed source carries the library styling (e.g. `active:translate-y-px` on buttons) rather than the plain shadcn fallback.
- Do not override component geometry on usage sites: className on `Button`, `Input`, `SelectTrigger`, and `Textarea` carries layout only (width, spacing, font). Radii, borders, and padding belong to the components so every control stays visually unified (`rounded-lg` across controls).
- Use shadcn/ui only for foundational primitives that Watermelon UI does not provide. Do not add another UI framework or build a duplicate component when either registry already covers the interaction.
- Keep constants such as API paths, enum values, and status codes under `src/lib/constants/`.
- Keep pure, exported utility functions under `src/lib/utils/` and cover them with unit tests.
- Read environment variables only through typed exports under `src/lib/env/`; components must not read environment variables directly.

## Routing

- Centralize route definitions in `src/router/index.ts` using
  `createBrowserRouter` with a `RouterProvider`; do not inline `<Routes>`
  in App components.
- Load page components with `React.lazy` per route so the initial bundle only
  contains the current page.
- Mount the route-level Error Boundary through each route's `errorElement`;
  feature-level boundaries still wrap independently failing sections.
- `App` remains a layout shell (navigation, error boundaries, route outlet).

## Errors and async states

- Place Error Boundaries at route and feature boundaries so one failure does not crash the whole application.
- Every asynchronous screen and action must handle loading, error, and success states explicitly.
- Never expose Secret values in errors, logs, query keys, browser storage, analytics, or development tooling.

## Tests

- Use Vitest for utilities and Hooks.
- Use React Testing Library for component rendering and interaction.
- Use MSW for API behavior in unit, Hook, and component tests.
- Use Playwright for critical browser workflows, including login and TOTP, Secret editing and Publish, Agent enrollment, failed Activation, and Rollback.
- Add integration coverage for module boundaries and all security-relevant workflows.

## Quality gates

Before merge, CI must run formatting or style checks, ESLint, TypeScript type checking, unit tests, component tests, integration tests, the production build, and the critical Playwright suite. Reviews must check regressions, security boundaries, accessibility, performance-sensitive rendering, reuse of existing modules, and documentation for non-obvious behavior.
