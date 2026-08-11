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
- Use [Watermelon UI](https://ui.watermelon.sh/home) as the primary component and block registry.
- Install Watermelon UI source through its documented shadcn registry URLs and treat the copied source as project-owned code subject to the same typing, accessibility, security, and testing rules.
- Use shadcn/ui only for foundational primitives that Watermelon UI does not provide. Do not add another UI framework or build a duplicate component when either registry already covers the interaction.
- Keep constants such as API paths, enum values, and status codes under `src/lib/constants/`.
- Keep pure, exported utility functions under `src/lib/utils/` and cover them with unit tests.
- Read environment variables only through typed exports under `src/lib/env/`; components must not read environment variables directly.

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
