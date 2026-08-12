# Record Rollback as a new Bundle Revision

Rollback will copy the Secret Version and File Binding selections of an earlier Bundle Revision into a new immutable Bundle Revision and make that new revision Desired State. Repointing Desired State directly at the historical row would obscure when and why the rollback occurred; a new revision preserves a linear publication history, supports the same per-Assignment Convergence flow as Publish, and gives Audit Events an unambiguous action result.
