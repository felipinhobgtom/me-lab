# ME Lab source-of-truth rule

For every implementation, bug fix, visual change, catalog entry, simulation rule, or other project task, MUST consult the version-matched Applied Energistics 2 Guide and/or Minecraft Wiki before changing code.

ME Lab targets Minecraft `1.21.1` and Applied Energistics 2 `19.x`. Implement documented Minecraft and AE2 behavior 1:1: mechanics, limits, recipes, block states, sidedness, power, channels, inventories, transitions, and failure behavior. Do not invent simulator behavior from memory or convenience.

Every AE2 block with interaction or configuration MUST implement its documented interaction, state transition, persistence, and failure behavior before it is marked `exact` or exposed as fully supported. A static catalog entry or visual placeholder is not an implementation. If a documented behavior is not implemented, mark that block/feature `unsupported` and document the gap; do not silently approximate it.

Every task MUST record the consulted source URLs and target version in the task/commit/architecture notes and MUST add or update deterministic tests for changed observable behavior. If the authoritative sources do not define a behavior or it cannot yet be implemented faithfully, mark it `unsupported` and document the gap; do not silently approximate it.

Primary sources:
- https://guide.appliedenergistics.org/1.21.1/index
- https://minecraft.wiki/

Consultation does not permit copying proprietary Minecraft or AE2 assets into the repository; use original/procedural visuals while matching the documented states and geometry.
