# Watermelon UI Component Adoption

Status: ready-for-agent

## Problem Statement

The frontend guidelines
([docs/frontend-guidelines.md](../../docs/frontend-guidelines.md)) require
Watermelon UI as the primary component and block registry, with shadcn/ui
only for foundational primitives Watermelon does not provide. The Web
application was built and refactored against every other guideline, but its
UI layer still uses shadcn primitives and plain HTML elements because the
Watermelon registry was unreachable at implementation time: its endpoints
returned SPA HTML instead of component JSON, so `shadcn add` could not
install anything. That fallback was recorded as a grilled decision in
[docs/implementation-plan.md](../../docs/implementation-plan.md) with the
explicit condition "revisit when the registry serves `index.json` /
component manifests".

Until the UI layer matches the guideline, every future screen either reuses
non-guideline components or duplicates interaction patterns that the
registry already covers.

## Solution

Establish an automated registry availability gate that verifies Watermelon
serves real registry manifests. When the gate passes, migrate the Web
application's UI layer to Watermelon components in batches: install each
component through the documented registry URL, replace the corresponding
shadcn primitive or plain element, and keep behavior, test ids, form
validation messages, and the E2E flows identical. shadcn primitives remain
only where Watermelon has no equivalent. When the gate fails, the migration
does not start and the current UI stays in place; the gate result is the
single source of truth for readiness.

## User Stories

1. As a developer, I want a script that verifies the Watermelon registry
   serves real component manifests, so that work only starts when the
   dependency is actually installable.
2. As a developer, I want the registry gate to distinguish a JSON manifest
   from an SPA HTML fallback, so that a reachable-but-broken registry never
   looks ready.
3. As a developer, I want to install Watermelon components through its
   documented registry URLs, so that copied source is treated as
   project-owned code subject to the same rules as hand-written components.
4. As a developer, I want a fixed component inventory for the migration, so
   that batches are scoped and reviewable.
5. As a developer, I want each migration batch to replace one interaction
   surface (buttons, inputs, selects, tables, dialogs) across the
   application, so that each batch is independently verifiable.
6. As an Administrator, I want every form to keep its current behavior,
   validation messages, and error display after the swap, so that the
   migration is invisible to me.
7. As an Administrator, I want the bootstrap, login, secret, binding,
   group, and assignment forms to keep the same test ids and submit
   semantics, so that automation and muscle memory keep working.
8. As an Administrator, I want the install command card to keep its
   one-time presentation and copy affordance, so that the enrollment flow
   is unchanged.
9. As an Administrator, I want the node table, group rows, and assignment
   list to render and update exactly as before, so that fleet operations
   are unaffected.
10. As a developer, I want shadcn primitives kept only when Watermelon has
    no equivalent, so that the component layer converges on one registry.
11. As a developer, I want every replaced component covered by a React
    Testing Library test, so that the component seam verifies rendering and
    interaction.
12. As a developer, I want the Playwright E2E suite to stay green after
    each batch, so that the full user workflow remains the behavioral
    proof.
13. As a developer, I want the migration to keep lint, typecheck, build,
    and the Vitest suite green after every batch, so that quality gates
    never regress silently.
14. As a developer, I want the registry gate wired into the repository so
    that a future agent can check readiness before starting, so that the
    deferred decision has an explicit re-entry condition.
15. As a developer, I want the migration to avoid touching Hooks, data
    fetching, stores, constants, or routing, so that the blast radius stays
    inside the UI layer.

## Implementation Decisions

- **Registry availability gate**: a script that requests the registry's
  manifest endpoint (the documented `index.json` / component manifest URL)
  and passes only when the response is JSON with the expected manifest
  shape. An HTML response, a 4xx/5xx, or a timeout fails the gate. The gate
  is the only readiness check; no migration work starts while it fails.
  This directly encodes the grilled fallback condition.
- **Component inventory**: the migration targets the interaction surfaces
  the application actually uses: button, input, select, form error text,
  table, dialog/toast, and any Watermelon equivalent of the existing
  shadcn primitives (button, button-group, accordion, separator). The
  inventory is fixed before the first batch; adding components mid-flight
  requires updating the inventory first.
- **Batch strategy**: one batch per interaction surface (e.g. all buttons
  and primary actions, then all inputs and selects, then tables and
  dialogs). Each batch swaps the component usage across every page that
  uses it, updates or adds the corresponding RTL tests, and runs the full
  quality gates plus the E2E suite before the next batch starts.
- **Behavior contract**: test ids, placeholder text, validation messages
  (Zod schemas are untouched), submit semantics, disabled states, and
  accessibility attributes must be identical after the swap. The RTL tests
  that currently assert on test ids and button states are the contract.
- **shadcn retention rule**: a shadcn primitive stays only when Watermelon
  provides no equivalent interaction. Each retention is recorded with the
  missing Watermelon component named, so the list shrinks over time.
- **No changes outside the UI layer**: Hooks, TanStack Query wrappers, the
  Zustand session store, `lib/api`, constants, schemas, routing, and the
  data model are out of the migration's reach. The swap is purely
  presentational.
- **Fallback stays documented**: while the gate fails, the existing UI and
  the grilled fallback note in the implementation plan remain the operative
  state; the spec's readiness gate supersedes manual retries.

## Testing Decisions

- **What makes a good test**: external behavior only — rendered structure,
  visible text, test ids, enabled/disabled states, form submission and
  validation output — never component internals. A swapped component is
  done when its RTL tests pass against the new implementation and the
  existing assertions still describe the user-visible contract.
- **Registry gate**: a plain script test that requests the manifest
  endpoint and asserts JSON shape; run manually and wired into CI as a
  non-blocking check so the readiness state is visible.
- **Component seam (prior art: `web/src/pages/nodes-page.test.tsx`,
  `bootstrap-page.test.tsx`)**: Vitest + React Testing Library + MSW for
  every replaced component. New tests are added for components that lacked
  coverage (binding row, assignment form, draft panel, install command
  card) using the existing MSW handlers in `src/test/server.ts`.
- **Behavior seam (prior art: `web/e2e/slice1.e2e.ts`)**: the Playwright
  suite is the single behavioral proof and must stay green after every
  batch — bootstrap, login, authoring, publish, assignment, install
  command, node fixture convergence, and file landing.
- **Quality gates**: lint, `tsc -b`, production build, and the full Vitest
  suite run after every batch, matching the project's common definition of
  done.

## Out of Scope

- Visual redesign, new interactions, theming, or motion work beyond what
  the Watermelon components render by default.
- Changes to Hooks, data fetching, stores, constants, schemas, routing, or
  the API transport.
- Watermelon adoption while the registry gate fails; the current UI stays
  operative and the grilled fallback remains in force.
- Replacing components the application does not use, or installing the
  entire registry speculatively.
- Backend, Agent, or deployment changes of any kind.

## Further Notes

- The registry gate re-entry condition was recorded during the frontend
  grilling session: "revisit when the registry serves `index.json` /
  component manifests". This spec operationalizes that condition as a
  checkable script instead of a manual retry.
- The frontend guidelines require copied registry source to be treated as
  project-owned code subject to typing, accessibility, security, and
  testing rules; each installed component therefore lands in the project
  tree with the same review bar as hand-written code.
- The component inventory should be re-validated against the actual pages
  at migration time; if a page needs an interaction the inventory lacks,
  update the inventory before the batch that touches that page.
