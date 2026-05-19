# Custom Tool & MCP Detail Renderers Design

## Background

Currently, the settings page (`SkillToolsConfigPage.tsx`) uses a single generic function `renderToolDetail` to render the configuration panel for **all** tools and MCP servers. This function relies on `settings_schema` to dynamically generate form fields (bool, int, secret, text).

While this approach is generic, it produces a mechanical and uninspired UI for built-in tools that have rich semantics. For example, `browser_control` — which enables bidirectional browser extension communication — is reduced to two plain fields (`enabled` toggle + `api_key` password input). There is no connection status, no test button, no install guide, and no visual hierarchy that communicates "this is a browser control panel."

The same limitation applies to MCP servers, whose detail panel currently shows only a toggle and a sentence: "MCP 服务器配置请在 Advanced 页面管理."

## Goals

1. Allow **every built-in tool** to have its own custom configuration UI when needed.
2. Allow **MCP servers** to have custom configuration UIs as well.
3. Keep the system **opt-in and backward-compatible**: tools without a custom renderer fall back to an enhanced generic schema renderer.
4. Establish a clean, extensible architecture so future tools can be added with minimal boilerplate.
5. Improve the aesthetics of both custom and fallback renderers.

## Architecture: Hybrid Approach

We adopt a **hybrid architecture** combining two mechanisms:

- **Component Slot Registry** (for tools/MCP that need fully custom UI + custom interactions).
- **Enhanced Schema-Driven Renderer** (for tools that only need richer form layouts without full custom components).

### Why not purely schema-driven?

A pure schema-driven approach (adding `ui_component` hints to `ConfigFieldDTO`) cannot express complex interactive behaviors such as:
- A "Test Connection" button that calls an API and updates local state.
- An install guide that conditionally appears based on a health check.
- A visual status card with icons and color-coded borders.

### Why not purely hard-coded?

A purely hard-coded approach (front-end maps tool names to React components) is perfectly fine for built-in tools, but we still want the generic fallback to be as capable as possible so simple tools get a good experience without requiring a custom component.

## Directory Structure

```
web/src/components/groups/skills/
├── SkillToolsConfigPage.tsx
├── SkillToolsConfigPage.module.css
├── detail-renderers/
│   ├── registry.ts              # Registration tables for tools and MCP
│   ├── types.ts                 # Shared Props interfaces
│   ├── ToolDetailFallback.tsx   # Enhanced generic schema renderer
│   ├── McpDetailFallback.tsx    # Enhanced generic MCP fallback
│   ├── browser/
│   │   ├── BrowserControlConfig.tsx
│   │   └── BrowserControlConfig.module.css
│   └── mcp/
│       └── (reserved for future MCP custom renderers)
```

## Registration Mechanism

`registry.ts` explicitly imports and maps tool names / MCP keys to React components:

```ts
import BrowserControlConfig from './browser/BrowserControlConfig';

export const toolDetailRegistry: Record<string, React.FC<ToolDetailProps>> = {
  browser_control: BrowserControlConfig,
  // Future: web_search: WebSearchConfig, file_system: FileSystemConfig, ...
};

export const mcpDetailRegistry: Record<string, React.FC<McpDetailProps>> = {
  // Future: filesystem: FilesystemMcpConfig, github: GithubMcpConfig, ...
};
```

`SkillToolsConfigPage.tsx` dispatches via the registry:

```tsx
const ToolRenderer = toolDetailRegistry[name] ?? ToolDetailFallback;
return <ToolRenderer ... />;

const McpRenderer = mcpDetailRegistry[mcpKey] ?? McpDetailFallback;
return <McpRenderer ... />;
```

**Principles:**
- **Zero intrusion**: unregistered tools automatically use the enhanced fallback.
- **Explicit registration**: every custom component is explicitly imported and listed. No magic strings, no dynamic imports.
- **Co-location**: custom components live next to the registry, grouped by tool name.

## Unified Props Interfaces

```ts
// detail-renderers/types.ts

export interface ToolDetailProps {
  name: string;
  description?: string;
  toolset?: string;
  enabled: boolean;
  onToggle: (nextEnabled: boolean) => void;
  config?: Record<string, unknown>;
  onSectionField: (sectionKey: string, field: string, value: unknown) => void;
}

export interface McpDetailProps {
  key: string;
  command: string;
  enabled: boolean;
  onToggle: (nextEnabled: boolean) => void;
  serverConfig: Record<string, unknown>;
  onServerChange: (next: Record<string, unknown>) => void;
}
```

**Rationale for passing the full `config` tree:**
- Custom components often need to read/write config sections outside their own namespace.
- Example: `browser_control`'s settings live under `browser_extension.api_key`, not `tools.settings.browser_control.api_key`.
- By passing the entire tree, custom components decide which section to mutate via `onSectionField`, keeping the data flow consistent with the rest of the settings page.

## `browser_control` Custom UI (First Example)

The `BrowserControlConfig` component replaces the generic two-field form with a rich, contextual control panel.

### Layout Sections

1. **Visual Header**
   - Large emoji/icon (🌐) + tool name + `(browser)` toolset tag.
   - One-line description explaining what browser control does.
   - Enable switch on the right, integrated into the header card.

2. **Connection Status Card**
   - A standalone card showing real-time extension health:
     - 🟢 Connected (with version, e.g. `v1.2.0`)
     - 🟡 Not configured (API Key missing)
     - 🔴 Extension not responding
   - Color-coded border and background using existing CSS variables (`--success`, `--warning`, `--error`).

3. **Authentication Section**
   - Grouped card labeled "Authentication".
   - Uses the existing `SecretInput` component (show/hide toggle, mono font).
   - Label: "Extension API Key".
   - Help text: "The browser extension uses this key to authenticate with Hermind."

4. **Action Bar**
   - **Test Connection**: calls `GET /api/browser-extension/check`, updates the status card above.
   - **Copy Key**: copies the current API key to clipboard for pasting into the extension settings.
   - Buttons styled consistently with the existing UI (border radius, hover states, mono font).

5. **Install Guide (conditional)**
   - Displayed when the extension is not detected.
   - Brief text guide + link to Chrome Web Store (or local unpacked install instructions).

### Data Writes

| User Action | Callback | Target Section | Target Field |
|-------------|----------|----------------|--------------|
| Toggle enable | `onToggle` | `browser_extension` | `enabled` |
| Change API Key | `onSectionField` | `browser_extension` | `api_key` |

### CSS Strategy

- Reuse existing classes from `SkillToolsConfigPage.module.css` (`.detailHeader`, `.detailTitle`, `.configSection`, `.warningBanner`).
- Local classes (`.statusCard`, `.actionBar`, `.authCard`) live in `BrowserControlConfig.module.css`.

## MCP Custom Rendering

MCP servers currently have no schema. Their detail panel is nearly empty. We apply the same registry pattern:

- `mcpDetailRegistry` maps server `key` → custom component.
- Useful for well-known MCP servers (`filesystem`, `github`, `fetch`, etc.) where we want a tailored UI (path pickers, token inputs, permission toggles).
- Unregistered MCP servers fall back to `McpDetailFallback`, which displays:
  - Server name and command (read-only).
  - Enable switch.
  - A link to the Advanced page for raw JSON editing.

**Future backend enhancement (out of scope for this design):** if the backend later exposes per-MCP schemas, the fallback renderer can be enhanced to render them similarly to `ToolDetailFallback`.

## Enhanced Generic Schema Renderer (`ToolDetailFallback`)

For tools and skills without a custom component, the fallback renderer is significantly improved over the current `renderSchemaFields`.

### Improvements

1. **Field Grouping**
   - Group fields by an optional `group` property in `ConfigField`.
   - Each group renders as a titled section (e.g., "Authentication", "Limits", "Proxy").
   - Ungrouped fields land in a default "General" section.

2. **Richer Field Type Support**
   - Route `kind` to existing dedicated components in `web/src/components/fields/`:
     - `bool` → `Switch`
     - `int` / `float` → `NumberInput` / `FloatInput`
     - `string` → `TextInput`
     - `secret` → `SecretInput`
     - `enum` → `EnumSelect`
     - `text` (multiline) → `TextAreaInput`
     - `multiselect` → `MultiSelectField`
   - Stop inlining raw `<input>` elements.

3. **Conditional Visibility (`visible_when`)**
   - The backend schema already defines `visible_when: ConfigPredicate`.
   - The fallback renderer evaluates these predicates against current field values to show/hide dependent fields.
   - Example: `proxy_url` only appears when `use_proxy === true`.

4. **Layout Polish**
   - For short inputs (`string`, `number`, `secret`, `enum`): label + help on the left, input on the right (two-column feel within the 720px max-width).
   - For `bool`: single row with label left, switch right.
   - For `text` (textarea): full-width stacked layout.

### Backward Compatibility

- `group` and `visible_when` are optional. If absent, the renderer behaves like today but with better styling.
- Backend can add these properties incrementally; the frontend already understands them.

## Error Handling & Fallback Strategy

| Scenario | Behavior |
|----------|----------|
| Tool name has custom registry entry | Render custom component |
| Tool name has no registry entry, has `settings_schema` | Render `ToolDetailFallback` |
| Tool name has no registry entry, no `settings_schema` | Render `ToolDetailFallback` with "No settings available" message |
| MCP key has custom registry entry | Render custom component |
| MCP key has no registry entry | Render `McpDetailFallback` |
| Registry lookup returns `undefined` at runtime | Fallback to default renderer (defensive) |

**Error Isolation:**
- Custom components manage their own local state (loading, error, test results).
- A failure inside `BrowserControlConfig` (e.g., network error on Test Connection) does not crash the parent settings page.

## Testing Strategy

### Unit Tests (Jest + React Testing Library)

1. **`registry.ts`**
   - Assert that expected tool names map to actual React components.
   - Prevent accidental deletion or rename drift.

2. **`ToolDetailFallback.tsx`**
   - Render with a mock `settings_schema` containing all `kind` types.
   - Assert correct field components are rendered.
   - Test `visible_when` show/hide logic.
   - Test `group` aggregation.

3. **`BrowserControlConfig.tsx`**
   - Toggle switch calls `onToggle` with correct value.
   - API Key change calls `onSectionField('browser_extension', 'api_key', value)`.
   - "Test Connection" button triggers `apiFetch` and updates status UI.
   - Install guide appears when connection status is negative.

### Integration Tests

- Update `SkillsSection.test.tsx` (or related existing tests):
  - Clicking `browser_control` in the tool list renders `BrowserControlConfig` instead of generic schema fields.
  - Clicking an unregistered tool still renders the fallback correctly.

## Migration Plan

1. Create `detail-renderers/` directory and scaffolding (`types.ts`, `registry.ts`).
2. Extract and enhance generic renderers into `ToolDetailFallback.tsx` and `McpDetailFallback.tsx`.
3. Refactor `SkillToolsConfigPage.tsx` to use the registry dispatch pattern.
4. Implement `BrowserControlConfig.tsx` as the first custom component.
5. Add unit tests for registry, fallback, and `BrowserControlConfig`.
6. Update integration tests in the skills section.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `web/src/components/groups/skills/detail-renderers/registry.ts` | **Create** | Registration tables |
| `web/src/components/groups/skills/detail-renderers/types.ts` | **Create** | Shared Props interfaces |
| `web/src/components/groups/skills/detail-renderers/ToolDetailFallback.tsx` | **Create** | Enhanced generic tool renderer |
| `web/src/components/groups/skills/detail-renderers/McpDetailFallback.tsx` | **Create** | Enhanced generic MCP renderer |
| `web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.tsx` | **Create** | Custom browser_control panel |
| `web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.module.css` | **Create** | Styles for browser_control panel |
| `web/src/components/groups/skills/SkillToolsConfigPage.tsx` | **Refactor** | Remove inline renderers, use registry |
| `web/src/components/groups/skills/SkillToolsConfigPage.test.tsx` (or related) | **Update** | Registry dispatch tests |
| `web/src/components/groups/skills/detail-renderers/*.test.tsx` | **Create** | New unit tests |

## Future Work (Out of Scope)

- Backend: expose `group` and `visible_when` in tool `settings_schema` responses (frontend already supports them once present).
- Backend: expose MCP server capability schemas so fallback renderer can auto-generate forms for arbitrary MCP servers.
- Custom renderers for additional built-in tools (`web_search`, `file_system`, etc.).
