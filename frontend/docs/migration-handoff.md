# Frontend Migration Handoff (shadcn/ui Alignment)

This note captures the current state of the shadcn/ui adoption effort so the next developer can pick up without re‑deriving context.

## Key References

- `docs/shadcn-mapping.md`: authoritative inventory of every component living under `src/components` and the recommended shadcn/ui replacements (sourced via the MCP shadcn registry and context7 docs).
- `src/components/ui/*`: legacy primitives slated for replacement (Button, Input, Select, etc.).
- `src/components/provider-symbol-picker.tsx`, `confirm-dialog.tsx`, etc.: higher-level widgets that will migrate after the primitives settle.

## Migration Priorities

1. **Primitives First**  
   Replace the low-level UI building blocks (button, input, textarea, checkbox, select, dropdown-menu, dialog, tabs, table, scroll-area, sheet, badge, alert) before touching feature components. This keeps downstream changes small and composable.
2. **Composite UI Next**  
   After primitives ship, move on to cards, charts, menus, ProviderSymbolPicker, ConfirmDialog, Nav, etc., wiring them to the new shadcn primitives.
3. **Infrastructure Last**  
   QueryProvider / QueryHydration / ClientProviders stay as-is. The only infra slated for change is theming: plan to replace the bespoke ThemeProvider logic with `use-dark-mode` + `theme-switcher` from shadcn, then retire the custom ThemeScript.

## How to Scaffold shadcn Components

For each component you intend to replace, run the CLI from the repo root:

```bash
npx shadcn@latest add <component>
```

Examples (in suggested order): `button`, `input`, `textarea`, `select`, `checkbox`, `badge`, `dropdown-menu`, `dialog`, `tabs`, `table`, `scroll-area`, `sheet`, `tooltip`, `form`, `combobox`, `code-editor`, `code-block`, `theme-switcher`, `theme-toggle-button`, `sonner`.

The CLI will place files under `components/ui/…`; update imports per `docs/shadcn-mapping.md`.

> **Directive**: The migration intentionally drops backward compatibility. Remove legacy components outright—no shim layers, adapters, or “compat” exports.

## Net-New Work

- **BackgroundCanvas**: Build a reusable background wrapper (e.g., marketing hero/dash surface) inspired by shadcn’s `background-beams`. This is a new component—not a replacement—and should expose props for density/speed so product teams can tune the visual intensity.

## Practical Next Steps

1. Install the first tranche of primitives via the CLI (button/input/select/etc.).
2. Swap existing `src/components/ui` imports to point at the generated shadcn versions, following the mapping doc.
3. Once primitives are migrated, tackle composite widgets and finally theme infrastructure.

With this doc plus `docs/shadcn-mapping.md`, the next developer has all the actionable context needed to continue the migration.***

## Kickoff Snapshot (week of November 13, 2025)

- `components.json` is already configured for the shadcn CLI (style `new-york`, RSC enabled), so no bootstrap work is required before running `npx shadcn@latest add …`.
- Legacy primitives still live under `src/components/ui/*.tsx`; every feature route (`src/app/(dashboard|instances|strategies|risk)`) imports from this folder, so each swap should be done behind a short-lived feature branch to avoid churn.
- Code surfaces (`src/components/code/*`) continue to depend on Ace via `react-ace`; replacing them with shadcn’s `code-editor`/`code-block` requires deleting the Ace loader and updating `CodeViewer` consumers (Instances, Strategy Modules) in lockstep.
- Theming now rides on `next-themes` + the custom palette context (`src/components/ui/theme-provider.tsx`, `theme-script.tsx`, `src/components/theme-toggle.tsx`), matching the shadcn `theme-switcher` UX while preserving Meltica’s palette metadata.
- Background visuals (Dashboard hero, empty states) do not yet share a reusable primitive; the brand/design team specifically asked for a `BackgroundCanvas` derived from shadcn’s `background-beams` once primitives are stable.

## Kickoff Checklist

1. **Freeze the current `src/components/ui` API surface.** Generate a dependency list with `rg "@/components/ui" -n src` so you know which screens to manually verify while swapping imports.
2. **Install the first tranche of primitives.** Run `npx shadcn@latest add button input textarea select checkbox dropdown-menu dialog tabs table scroll-area sheet badge alert` from the repo root. Commit each tranche so regressions are easy to bisect.
3. **Wire utility helpers.** Confirm `@/lib/utils` still exports `cn` and that the `tailwind.config` preset matches the `components.json` style so newly generated components pick up the right tokens.
4. **Swap imports per mapping.** Follow the `docs/shadcn-mapping.md#component-inventory` table, deleting each legacy file right after the last import switch to avoid dual sources of truth.
5. **Re-test the critical flows.** After every tranche, run `pnpm lint`, `pnpm test`, and smoke the Instances + Strategies routes in `pnpm dev` to surface hydration issues early.
6. **Document deltas.** Update both docs (this handoff and the mapping) with the actual completion dates so downstream contributors know which layers are safe to build on.

## Workstream Tracker

| Workstream | Scope | Status | Next Deliverable | Key Files |
| --- | --- | --- | --- | --- |
| Primitive swap | Import + replace button/input/textarea/select/checkbox/badge/alert/dropdown-menu/dialog/tabs/table/scroll-area/sheet/form/chart helpers. | In progress — buttons/inputs + alert/badge/tooltip/toast + dialog/dropdown-menu/sheet/tabs/scroll-area/separator + form/table/chart updated on Nov 13, 2025. | Spot-check routes for straggling legacy imports (card/chart consumers, virtualized scroll props) before moving on to composite widgets. | `src/components/ui/*.tsx`, `src/app/**/*` |
| Composite widgets | Nav, ProviderSymbolPicker, ConfirmDialog, charts, sheets, sheets-with-forms. | Completed (Nov 13, 2025) — Nav now mirrors `navbar-05`, ConfirmDialog uses dialog-stack, and ProviderSymbolPicker adopts the combobox/tags UX with virtualized search. | Keep an eye on future feature widgets to ensure they build atop the new primitives. | `src/components/nav.tsx`, `src/components/provider-symbol-picker.tsx`, `src/components/confirm-dialog.tsx` |
| Theming refresh | Replace ThemeProvider/ThemeScript with `use-dark-mode` + shadcn theme switcher. | Completed (Nov 13, 2025) — NextThemes-backed provider + combobox theme switcher shipped. | Keep palette additions (`theme-config.ts`) and theme provider tests (`theme-provider.test.tsx`) updated when new palettes land. | `src/components/ui/theme-provider.tsx`, `src/components/theme-toggle.tsx` |
| Code surfaces | Replace Ace-based editor/viewer with `code-editor` + `code-block`. | Completed (Nov 13, 2025) — Ace/React-Ace removed in favor of the shadcn-style CodeMirror implementation. | Monitor Strategy Modules + Context editors for any regressions and consider adding syntax validation extensions. | `src/components/code/*.tsx` |
| BackgroundCanvas (net-new) | Create reusable animated backdrop based on `background-beams`. | Completed (Nov 13, 2025) — `BackgroundCanvas` exposes density/speed controls for hero/dash wrappers. | Encourage feature teams to wrap hero panels and dashboards with the new component for consistent ambiance. | `src/components/ui/background-canvas.tsx` |

## Risks & Mitigations

- **Ace removal is invasive.** Strategy Modules, Context backup, and any code diff viewers depend on Ace-specific APIs. Mitigation: land primitives first, then tackle the code editor behind a `feature/code-editor-shadcn` branch with MSW-backed snapshot tests.
- **Theme regression potential.** `next-themes` now owns preference storage; keep `src/components/ui/theme-provider.test.tsx` in sync whenever palette IDs or dataset attributes change so regressions surface early.
- **Momentum loss.** Without a lightweight burndown, contributors may not know what to tackle next. Keep the tracker table above up to date at the end of each working session.

## Definition of Done for Kickoff

- All high-priority primitives listed in the checklist are sourced from shadcn CLI and no longer reference the legacy Radix wrappers under `src/components/ui`.
- The mapping doc reflects the true state (statuses updated, legacy files removed from the inventory once deleted).
- `pnpm lint`, `pnpm test`, and a manual Instances/Strategies smoke test pass without visual regressions.
- A short Loom or screenshot bundle is attached to the eventual PR to document any intentional visual changes for design review.
