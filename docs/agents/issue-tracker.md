# Issue Tracker: Local Markdown

Issues and specs for this repository live as Markdown files under `.scratch/` because the repository has no configured remote issue tracker.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`
- The feature specification is `.scratch/<feature-slug>/spec.md`
- Implementation issues are one file per ticket under `.scratch/<feature-slug>/issues/`, numbered from `01`
- Triage state is a `Status:` line near the top of each issue file
- Comments and conversation history append under a `## Comments` heading

## Publishing

When a skill says to publish to the issue tracker, create or update the corresponding Markdown file under `.scratch/<feature-slug>/`. Do not publish to the legacy `autosecrets` Gitea repository unless this configuration is explicitly changed.
