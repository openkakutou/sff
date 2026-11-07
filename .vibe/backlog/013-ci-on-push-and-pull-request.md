---
status: todo
---
# Run CI On Push And Pull Request

## Description
Run build, test, vet and the WASM build on every push and PR, not only on tags.

## Acceptance Criteria
- [ ] A workflow runs build, test and vet on every push and pull request, not only on version tags
- [ ] A failing test makes the workflow fail, visible on the commit and PR
- [ ] The tag-triggered release workflow is unchanged and still publishes its assets

## Notes
Today only .github/workflows/release.yml exists, triggered by tags, so a regression is discovered at release time. Follow the pinned-by-SHA convention of web-ui-kit's workflows. Reuse the release job's steps (including the WASM build) rather than duplicating logic where a reusable workflow fits.
