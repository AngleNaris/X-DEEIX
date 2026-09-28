import assert from "node:assert/strict";
import test from "node:test";
import { THEME_PRESETS, THEMES } from "./theme.ts";

test("theme catalog preserves upstream presets and custom Artilus", () => {
  for (const preset of ["default", "azure", "cobalt", "graphite", "lagoon", "ink", "ochre", "sepia", "artilus"]) {
    assert.ok(THEME_PRESETS.includes(preset), `missing preset ${preset}`);
  }
  assert.equal(new Set(THEME_PRESETS).size, THEME_PRESETS.length);
  assert.deepEqual(THEMES, ["light", "dark", "system"]);
});
