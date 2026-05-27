# shadcn/ui Replacement Mapping

This document inventories every component living under `src/components` and pairs it with the closest shadcn/ui building blocks so the team can migrate incrementally. The recommendations lean on the official shadcn registry (via the MCP `shadcn` server) and the public documentation surfaced through the `context7` integration.

## Component Inventory

| Component | Path | Responsibility |
| --- | --- | --- |
| Ace asset loader | `src/components/code/ace-loader.ts` | Lazy-loads Ace editor themes/modes/extras before mounting the editor. |
| CodeEditor | `src/components/code/code-editor.tsx` | Theme-aware Ace editor wrapper with shortcut registration and accessibility plumbing. |
| CodeViewer | `src/components/code/code-viewer.tsx` | Read-only façade over `CodeEditor` that disables editing and exposes copy callbacks. |
| Code exports | `src/components/code/index.ts` | Barrel file exporting the code-related primitives. |
| ConfirmDialog | `src/components/confirm-dialog.tsx` | App-wide confirmation dialog with error banner + scrollable body. |
| Nav | `src/components/nav.tsx` | Top navigation bar with theme toggle and route links. |
| ProviderSymbolPicker | `src/components/provider-symbol-picker.tsx` | Virtualized multiselect for provider instruments with filtering and metadata tooltips. |
| ClientProviders | `src/components/providers/client-providers.tsx` | Composes Query, Theme, Toast providers on the client. |
| QueryHydration | `src/components/providers/query-hydration.tsx` | Thin wrapper around `HydrationBoundary` for React Query. |
| QueryProvider | `src/components/providers/query-provider.tsx` | Initializes and exposes a `QueryClientProvider` plus devtools toggle. |
| ThemeToggle | `src/components/theme-toggle.tsx` | Dropdown that switches palette + theme preference. |
| Alert | `src/components/ui/alert.tsx` | Feedback block with semantic variants (default/destructive/etc.). |
| Badge | `src/components/ui/badge.tsx` | Pill-style status/tag indicator with multiple variants. |
| Button | `src/components/ui/button.tsx` | Variant-aware CTA/button primitive with cva-powered styling. |
| Card | `src/components/ui/card.tsx` | Container shell implementing card header/content/footer. |
| Chart helpers | `src/components/ui/chart.tsx` | Stacked bar chart + legend for inline analytics. |
| Checkbox | `src/components/ui/checkbox.tsx` | Theme-colored checkbox input. |
| Dialog primitives | `src/components/ui/dialog.tsx` | Radix dialog wrappers. |
| DropdownMenu | `src/components/ui/dropdown-menu.tsx` | Radix dropdown wrappers used across the app. |
| Form | `src/components/ui/form.tsx` | React Hook Form adapters (Form, FormField, FormItem, etc.). |
| Input | `src/components/ui/input.tsx` | Text input primitive with validation + selection styling. |
| Label | `src/components/ui/label.tsx` | Typography/alignment helper for form labels. |
| ScrollArea | `src/components/ui/scroll-area.tsx` | Radix scrollable container + optional bar. |
| Select | `src/components/ui/select.tsx` | Radix select wrappers. |
| Separator | `src/components/ui/separator.tsx` | Horizontal/vertical dividers. |
| Sheet | `src/components/ui/sheet.tsx` | Sliding drawer primitive. |
| Table primitives | `src/components/ui/table.tsx` | Basic table elements (Table, Row, Cell, etc.). |
| Tabs | `src/components/ui/tabs.tsx` | Tabs with animated motion highlight. |
| Textarea | `src/components/ui/textarea.tsx` | Multiline text input. |
| Theme config | `src/components/ui/theme-config.ts` | Palette metadata (ids, labels, source URLs). |
| ThemeProvider test | `src/components/ui/theme-provider.test.tsx` | Vitest coverage for the theme context. |
| ThemeProvider | `src/components/ui/theme-provider.tsx` | Controls light/dark/system state and palette persistence. |
| ThemeScript | `src/components/ui/theme-script.tsx` | Injects inline script to sync theme before hydration. |
| ToastProvider | `src/components/ui/toast-provider.tsx` | Local toast queue rendering portal. |
| Tooltip | `src/components/ui/tooltip.tsx` | Radix tooltip wrappers. |

## Proposed shadcn/ui Replacements

| Current Component | shadcn/ui Building Blocks | Migration Notes |
| --- | --- | --- |
| Ace asset loader | `code-editor` registry component | Shadcn’s `code-editor` already lazy-loads Shiki themes and handles copy buttons, so you can retire the Ace-specific loader by swapping to the registry version. |
| CodeEditor | `code-editor` + `code-block` | Replace Ace with the motion/Shiki-based `code-editor` for editing, and reuse `code-block` for static previews to keep both authoring and playback consistent. |
| CodeViewer | `code-block` | The registry `code-block` ships language badges, copy buttons, and Shiki highlighting, covering the viewer use case out of the box. |
| Code exports | `code-editor`, `code-block` | Barrel can re-export the two shadcn primitives once migrated. |
| ConfirmDialog | `dialog-stack`, `announcement` | `dialog-stack` gives stacked modal flows with triggers and overlays, while `announcement` covers the inline destructive alert currently rendered inside the dialog. |
| Nav | `navbar-05` | `navbar-05` already handles responsive collapse, badges, and dropdown user menus, so it can replace the handcrafted navigation bar. |
| ProviderSymbolPicker | `combobox`, `tags`, `scroll-area` | Use `combobox` for search-driven selection, `scroll-area` for the virtualized list container, and `tags` for rendering the “selected symbols” pill strip. |
| ClientProviders | — (infrastructure) | Keep as-is; no shadcn analogue is required because it wires query/theme/toast providers. |
| QueryHydration | — (infrastructure) | Continue using React Query’s `HydrationBoundary`; shadcn does not ship data-layer helpers. |
| QueryProvider | — (infrastructure) | Same rationale—remains a TanStack Query concern. |
| ThemeToggle | `theme-switcher`, `theme-toggle-button` | The registry exposes both a pill-style `theme-switcher` and an animated `theme-toggle-button` with view-transition flourishes—pair either with your palette menu. |
| Alert | `announcement` | The badge-backed `announcement` component provides the same semantic variants with lighter markup. |
| Badge | `badge` docs + `tags` | Standard shadcn `Badge` handles static pills; use the richer `tags` registry pattern when you need removable chips like the selected symbols list. |
| Button | `button` docs + `corner-accent-button` | The canonical shadcn `Button` covers everyday variants, while `corner-accent-button` offers a pre-built flashy CTA for marketing surfaces. |
| Card | `card` | Adopt the official `Card` sections (Header/Content/Footer) to stay aligned with shadcn typography spacing. |
| Chart helpers | `bar-chart-04` | The registry’s `bar-chart-04` pairs `recharts` with the `ChartContainer` helper, matching your stacked bar visualization with better tooltip and legend wiring. |
| Checkbox | `checkbox` docs + `choicebox` | Basic checkboxes map 1:1; for richer card-style multi-selects, the `choicebox` radio/checkbox hybrid replicates your current selectable rows. |
| Dialog primitives | `dialog-stack` | Consolidate on the motion-enabled stackable dialogs rather than maintaining your own wrappers. |
| DropdownMenu | `dropdown-menu` docs + `menu-dock` | For standard menus lean on the documented `DropdownMenu`; if you need icon-only grouped menus, `menu-dock` gives a polished dock-style experience. |
| Form | `form` docs | React Hook Form adapters in shadcn already expose the same `Form`, `FormField`, `FormItem`, etc., so you can drop the local copy. |
| Input | `input` docs + `input-button` | Replace the primitive with the shadcn `Input`, and reach for `input-button` when you need the “expandable input + submit chip” UX currently built bespoke. |
| Label | `label` docs | The official `Label` carries all the accessibility affordances; no need for a local wrapper. |
| ScrollArea | `scroll-area` docs (optionally `scroll-velocity` for marquee effects) | Use the stock Radix-based ScrollArea for virtualized panels; add `scroll-velocity` if you want kinetic, text-driven scrollers. |
| Select | `select` docs | Shadcn’s select includes grouping, search, and keyboard support—drop the custom Radix wrapper. |
| Separator | `separator` docs | Swap to the documented Separator so horizontal and vertical dividers stay in sync with sidebar/menu patterns. |
| Sheet | `sheet` docs | Adopt the shadcn Sheet (with `side` prop) for drawers instead of keeping a fork. |
| Table primitives | `table` docs + `table` registry component | The docs cover simple tables, while the registry `table` adds sortable headers and dropdown-driven controls similar to your TanStack integration. |
| Tabs | `tabs` registry component | The registry version already includes animated highlights and motion-driven content panels, replacing your local motion logic. |
| Textarea | `textarea` docs | Direct swap; keep the Tailwind sizing but lean on the upstream component for focus ring consistency. |
| Theme config | `theme-switcher` metadata | The registry theme components already expect palette metadata; you can migrate `theme-config` into the new switcher’s dataset or keep it as a data module. |
| ThemeProvider test | `use-dark-mode` hook (unit tests) | Once you move to `use-dark-mode` + `ThemeSwitcher`, update or remove the bespoke tests accordingly. |
| ThemeProvider | `use-dark-mode`, `theme-switcher` | Combine the lightweight `use-dark-mode` hook with `ThemeSwitcher`/`ThemeToggleButton` instead of maintaining manual localStorage/media-query syncing. |
| ThemeScript | `theme-toggle-button` (view transitions) | The registry toggle injects the necessary view-transition CSS, letting you delete the inline script. |
| ToastProvider | `sonner` (`toast` + `Toaster`) | Replace the custom provider with shadcn’s Sonner wrapper to get promise toasts, variants, and focus management. |
| Tooltip | `tooltip` docs + `animated-tooltip` | For standard tooltips use the documented primitive; when you need avatar previews like the picker’s info icon, `animated-tooltip` supplies motion + media slots. |

## Kickoff Priority Board

| Order | Component(s) | Source Paths | CLI Command(s) / Registry Blocks | Status & Notes |
| --- | --- | --- | --- | --- |
| 1 | Buttons & text inputs (`Button`, `Input`, `Textarea`, `Select`, `Checkbox`, `Label`) | `src/components/ui/button.tsx`, `input.tsx`, `textarea.tsx`, `select.tsx`, `checkbox.tsx`, `label.tsx` | `npx shadcn@latest add button input textarea select checkbox label` | **Completed – Nov 13, 2025.** CLI-generated components now back the primitives, and call sites were updated to use `onCheckedChange` for the new Radix Checkbox API. Smoke the providers/instances/module forms to validate toggles after this swap. |
| 2 | Feedback primitives (`Alert`, `Badge`, `ToastProvider`, `Tooltip`) | `src/components/ui/alert.tsx`, `badge.tsx`, `toast-provider.tsx`, `tooltip.tsx` | `npx shadcn@latest add alert badge tooltip sonner` | **Completed – Nov 13, 2025.** Alerts/badges now use the registry styles with our extended semantic variants, tooltips retain data-slot hooks, and the legacy toast queue was replaced with Sonner (`<Toaster />` + `useToast` shim). |
| 3 | Overlay & menu primitives (`Dialog`, `DropdownMenu`, `Sheet`, `Tabs`, `ScrollArea`, `Separator`) | `src/components/ui/dialog.tsx`, `dropdown-menu.tsx`, `sheet.tsx`, `tabs.tsx`, `scroll-area.tsx`, `separator.tsx` | `npx shadcn@latest add dialog dropdown-menu sheet tabs scroll-area separator` | **Completed – Nov 13, 2025.** CLI components are in place; Dialog/Sheet retain the custom close-button controls and ScrollArea keeps the `allowXScroll/allowYScroll` props so existing forms and virtualized lists behave the same. |
| 4 | Table & chart helpers (`Table`, `ChartContainer`) | `src/components/ui/table.tsx`, `chart.tsx` | `npx shadcn@latest add table` + copy `bar-chart-04` from the registry | **Completed – Nov 13, 2025.** Table now wraps the CLI container while preserving `containerClassName`, and `chart.tsx` hosts the Recharts-based stacked bar helper (`bar-chart-04`) so dashboards can show richer tooltips/legends. |
| 5 | Forms (`Form` + RHF adapters) | `src/components/ui/form.tsx` | `npx shadcn@latest add form` | **Completed – Nov 13, 2025.** RHF adapters match the registry defaults while keeping our `data-slot` hooks, so route-level forms can swap primitives drop-in. |
| 6 | Navigation + pickers (`Nav`, `ProviderSymbolPicker`, `ConfirmDialog`) | `src/components/nav.tsx`, `provider-symbol-picker.tsx`, `confirm-dialog.tsx` | Compose `navbar-05`, `combobox`, `tags`, `dialog-stack` manually (no single CLI command) | **Completed – Nov 13, 2025.** Nav mirrors `navbar-05`, ConfirmDialog uses dialog-stack, and ProviderSymbolPicker now wraps the virtualized search inside a shadcn-style combobox with removable tags. |
| 7 | Theming (`ThemeProvider`, `ThemeScript`, `ThemeToggle`, `theme-config.ts`) | `src/components/ui/theme-provider.tsx`, `theme-script.tsx`, `theme-config.ts`, `src/components/theme-toggle.tsx` | Import `use-dark-mode`, `theme-switcher`, `theme-toggle-button` from the registry | **Completed – Nov 13, 2025.** Reintroduced `next-themes`/`use-dark-mode`, rewrote ThemeProvider + palette context, simplified ThemeScript, and rebuilt ThemeToggle with the shadcn `theme-switcher` command palette UI. |
| 8 | Code surfaces (`CodeEditor`, `CodeViewer`, `Ace` loader) | `src/components/code/*.tsx` | Copy `code-editor` + `code-block` registry entries (manual integration) | Investigation needed. Replacing Ace touches CLI editor shortcuts and the Strategy Module diff view. Plan a feature branch and add snapshot tests around the execution workflow before cutting over. |
| 9 | BackgroundCanvas (net-new) | `src/components/ui/background-canvas.tsx` | Start from `background-beams` registry component | **Completed – Nov 13, 2025.** `BackgroundCanvas` derives from background-beams and exposes density/speed/blur props for hero and dashboard wrappers. |

### Implementation Notes

- Run each CLI command from the repo root so files land under `src/components/ui` per `components.json`.
- After each tranche, update import paths using `rg -l "@/components/ui/<name>"` to confirm nothing references the deprecated component before deleting it.
- The Sonner migration requires updating `ClientProviders` to expose the new `<Toaster />`; keep the existing toast hook shape so downstream modules stay untouched.
- Registry components such as `navbar-05` and `background-beams` ship as snippets rather than CLI commands—add them under `src/components/` (not `ui/`) to keep primitives vs composites separated.

## Net-New Components to Build

| Component | Reason to Add | shadcn/ui Source |
| --- | --- | --- |
| BackgroundCanvas (new) | The current codebase lacks a reusable surface/hero background. Create a new `BackgroundCanvas` component (rather than replacing anything) that can wrap dashboards or marketing panels. Start from the shadcn `background-beams` registry component to get the animated gradient particle effect, and expose props for density/speed so product teams can dial the visual intensity per page. **Do not introduce shim layers or backward-compat wrappers—use the new implementation directly.** | `background-beams` |
