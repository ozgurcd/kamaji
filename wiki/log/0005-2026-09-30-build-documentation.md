## [2026-09-30] docs | source-tree build system

Updated the live README, usage, CLI, language, lifecycle, limits, security,
release, testing, and example guides to distinguish unreleased graph builds
from the published v0.2.0 rule runner. Added an agent JSON workflow reference
and a complete compiled-Go graph walkthrough. The local wiki links both.

The documented plan/build/history wrapper and Go cache-restoration walkthrough
passed against the compiled CLI; the focused Go graph integration test passed.
The testing guide records those checks separately from the preceding full
requirements recheck. No product source, extension, or historical release note
was changed by this documentation slice. No commit or release was created;
the graph implementation remains working-tree work based on `18bd686b43cdc7395b6698bced041b008d830ae5`.
