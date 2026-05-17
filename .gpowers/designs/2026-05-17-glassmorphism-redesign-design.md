# Hermind Desktop UI Glassmorphism Redesign

## Overview

Redesign the entire Hermind desktop application UI to adopt a **layered depth glassmorphism** aesthetic. The design introduces a "depth hierarchy" where elements closer to the user appear more translucent with stronger light reflections, creating a modern, tech-forward visual language inspired by Raycast, macOS Control Center, and Vercel AI SDK Playground.

**Scope**: All QML UI components — Chat mode, Settings mode, TopBar, and global theme system.

**Constraint**: No new Qt modules. All effects implemented with pure QML (gradients, opacity, borders, layered Rectangles).

---

## 1. Visual Architecture: Depth Layers

The UI is organized into three depth layers, each with distinct glass properties:

| Layer | Elements | Opacity | Light Reflection | Border |
|-------|----------|---------|------------------|--------|
| **Background** | App window background | 100% (opaque gradient) | None | None |
| **Mid-layer** | TopBar, SettingsSidebar, ConversationHeader | 70-85% | Subtle top glow (5% height) | 1px semi-transparent |
| **Top-layer** | MessageBubble, PromptInput, SettingsCard, EmptyState prompts | 60-75% | Strong top highlight (10% height) | 1px semi-transparent + optional inner glow on focus |

### Background Layer

Replace the flat `Theme.bg` solid color with a **subtle radial gradient** that creates a sense of infinite depth:

- **Dark mode**: Center `#0d1117` (slightly blue-purple) → edges `#0a0b0d` (near black)
- **Light mode**: Center `#fafaf7` → edges `#f0ede4` (slightly warmer)

The gradient is applied to the root `ApplicationWindow` background via a `Rectangle` with `gradient: RadialGradient`.

---

## 2. Theme System Extensions

Add the following properties to `Theme.h` (C++ singleton) and expose them to QML:

| Property | Dark Value | Light Value | Purpose |
|----------|------------|-------------|---------|
| `glassBg` | `#14161a` @ 80% opacity | `#ffffff` @ 75% opacity | Mid-layer glass base |
| `glassBgLight` | `#1d2027` @ 60% opacity | `#ffffff` @ 90% opacity | Top-layer glass highlight |
| `glassBorder` | `#2a2e36` @ 50% opacity | `#d9d4c8` @ 40% opacity | Glass panel border |
| `glassSurface` | `#14161a` @ 85% opacity | `#f0ede4` @ 85% opacity | Sidebar / TopBar base |
| `glassSurfaceHover` | `#1d2027` @ 90% opacity | `#e8e4d6` @ 90% opacity | Sidebar item hover |
| `glassCard` | `#1a1c21` @ 70% opacity | `#f5f3ed` @ 70% opacity | Settings cards / message bubbles |
| `glassCardHover` | `#1d2027` @ 80% opacity | `#e8e4d6` @ 80% opacity | Card hover state |
| `glassInput` | `#14161a` @ 75% opacity | `#ffffff` @ 80% opacity | Input field background |
| `glassGlow` | `#FFB800` @ 30% opacity | `#FFB800` @ 25% opacity | Accent focus glow |
| `glassShadow` | `#000000` @ 20% opacity | `#000000` @ 10% opacity | Subtle drop shadow for top-layer elements |

**Implementation note**: Since QML `color` type does not natively support alpha in hex strings like `#RRGGBBAA` in all Qt versions, implement these as separate `color` + `real` (opacity) pairs, or use `Qt.rgba(r, g, b, a)` in QML. The cleanest approach: expose `color` values (opaque) and a companion `real` opacity property for each glass color, then set both `color` and `opacity` on the Rectangle.

**Simpler approach**: Just expose the ARGB hex colors as strings (Qt 6 supports `#AARRGGBB` format in many contexts). Or expose them as `QColor` with alpha from C++.

Actually, the simplest and most reliable: keep the existing opaque colors and add opacity constants. In QML, use `Rectangle { color: Theme.glassCard; opacity: Theme.glassCardOpacity }`.

So the actual Theme additions:

| Property | Type | Dark | Light |
|----------|------|------|-------|
| `glassBg` | color | `#14161a` | `#ffffff` |
| `glassBgOpacity` | real | 0.80 | 0.75 |
| `glassBgLight` | color | `#1d2027` | `#ffffff` |
| `glassBgLightOpacity` | real | 0.60 | 0.90 |
| `glassBorder` | color | `#2a2e36` | `#d9d4c8` |
| `glassBorderOpacity` | real | 0.50 | 0.40 |
| `glassSurface` | color | `#14161a` | `#f0ede4` |
| `glassSurfaceOpacity` | real | 0.85 | 0.85 |
| `glassSurfaceHover` | color | `#1d2027` | `#e8e4d6` |
| `glassSurfaceHoverOpacity` | real | 0.90 | 0.90 |
| `glassCard` | color | `#1a1c21` | `#f5f3ed` |
| `glassCardOpacity` | real | 0.70 | 0.70 |
| `glassCardHover` | color | `#1d2027` | `#e8e4d6` |
| `glassCardHoverOpacity` | real | 0.80 | 0.80 |
| `glassInput` | color | `#14161a` | `#ffffff` |
| `glassInputOpacity` | real | 0.75 | 0.80 |
| `glassGlow` | color | `#FFB800` | `#FFB800` |
| `glassGlowOpacity` | real | 0.30 | 0.25 |
| `glassShadow` | color | `#000000` | `#000000` |
| `glassShadowOpacity` | real | 0.20 | 0.10 |
| `bgGradientCenter` | color | `#0d1117` | `#fafaf7` |
| `bgGradientEdge` | color | `#0a0b0d` | `#f0ede4` |

Also add helper methods or just let QML compose the glass effect. A reusable `GlassPanel` component would be ideal.

---

## 3. Reusable Components

### `GlassPanel.qml` (new)

A reusable glass panel component that encapsulates the glass effect:

```qml
// Parameters:
// - layer: "mid" | "top"  (determines opacity and highlight strength)
// - radius: corner radius
// - hasBorder: bool
// - hasHighlight: bool
// - highlightColor: color for top glow

Rectangle {
    property string layer: "top"
    property real highlightStrength: layer === "top" ? 0.15 : 0.08
    property real baseOpacity: layer === "top" ? Theme.glassCardOpacity : Theme.glassSurfaceOpacity
    property color baseColor: layer === "top" ? Theme.glassCard : Theme.glassSurface
    
    color: baseColor
    opacity: baseOpacity
    radius: 8
    
    // Top highlight line (simulates light reflection)
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: parent.radius
        radius: parent.radius
        color: Theme.glassBgLight
        opacity: highlightStrength
        // Clip to top half
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height / 2
            color: parent.parent.color
            opacity: 1.0
        }
    }
    
    // Border
    Rectangle {
        anchors.fill: parent
        color: "transparent"
        radius: parent.radius
        border.color: Theme.glassBorder
        border.width: 1
        opacity: Theme.glassBorderOpacity
    }
}
```

Actually, a cleaner approach without nested opacity issues: use a single Rectangle with a linear gradient for the background, and an inner Rectangle for the border.

```qml
Rectangle {
    id: root
    property real glassOpacity: 0.75
    property real highlightStrength: 0.1
    property real borderOpacity: 0.5
    property real cornerRadius: 8
    
    // Base glass + top highlight in one gradient
    gradient: Gradient {
        GradientStop { position: 0.0; color: Qt.rgba(...highlight color...) }
        GradientStop { position: 0.08; color: Qt.rgba(...base color...) }
        GradientStop { position: 1.0; color: Qt.rgba(...base color...) }
    }
    opacity: glassOpacity
    radius: cornerRadius
    
    // Border overlay
    Rectangle {
        anchors.fill: parent
        color: "transparent"
        radius: parent.radius
        border.color: Theme.glassBorder
        border.width: 1
        opacity: borderOpacity
    }
}
```

But since gradients with `Qt.rgba` from C++ properties can be verbose, maybe keep it simple: just use `color` + `opacity` on the main Rectangle, and add a thin top highlight Rectangle as a child.

Let's go with the simple approach: main Rectangle with `color` and `opacity`, plus a child highlight Rectangle at the top.

### `GlassInput.qml` (new) — or modify existing fields

Input fields get a glass treatment:
- Background: `Theme.glassInput` @ `Theme.glassInputOpacity`
- Border: 1px `Theme.glassBorder` @ `Theme.glassBorderOpacity`
- Focus: border changes to `Theme.accent` + subtle outer glow (a larger Rectangle behind with `Theme.glassGlow` @ low opacity, blurred if possible, or just a solid low-opacity Rectangle)
- Since we can't blur, the "glow" is a larger Rectangle behind the input with `color: Theme.glassGlow`, `opacity: 0.15`, `radius: inputRadius + 2`

---

## 4. Chat Mode Redesign

### 4.1 AppWindow Background

Replace `Rectangle { color: Theme.bg }` in `AppWindow.qml` with a radial gradient:

```qml
Rectangle {
    // Radial gradient background
    gradient: RadialGradient {
        centerX: parent.width / 2
        centerY: parent.height / 2
        centerRadius: Math.max(parent.width, parent.height) * 0.8
        focalX: centerX
        focalY: centerY
        GradientStop { position: 0; color: Theme.bgGradientCenter }
        GradientStop { position: 1; color: Theme.bgGradientEdge }
    }
}
```

### 4.2 TopBar

Current: solid `Theme.surface` with 1px bottom border.

New:
- Background: `GlassPanel { layer: "mid"; radius: 0 }` — but TopBar has no radius on left/right edges, just a subtle bottom border
- Actually, keep it simple: `color: Theme.glassSurface; opacity: Theme.glassSurfaceOpacity`
- Bottom border: 1px `Theme.glassBorder` @ `Theme.glassBorderOpacity`
- Top highlight: a thin Rectangle at the very top, 1-2px, `Theme.glassBgLight` @ `0.1` opacity

### 4.3 ConversationHeader

Current: solid `Theme.surface`.

New: `color: Theme.glassSurface; opacity: Theme.glassSurfaceOpacity`. Remove bottom border (or make it glass border).

### 4.4 MessageBubble

Current:
- User: solid `Theme.accent`
- AI: solid `Theme.cardBg` with 1px `Theme.border`

New:
- **AI messages**: `GlassPanel { layer: "top"; cornerRadius: 12 }`
  - Base: `Theme.glassCard` @ `Theme.glassCardOpacity`
  - Border: `Theme.glassBorder` @ `Theme.glassBorderOpacity`
  - Top highlight for glass feel
  - Text stays `Theme.textPrimary`
  - Role label badge background: `Theme.glassSurface` @ `0.9`
  
- **User messages**: Accent-colored glass
  - Base: `Theme.accent` but at lower opacity (e.g., `0.25`)
  - Border: `Theme.accent` @ `0.4`
  - Top highlight: lighter tint of accent
  - Text stays `#1a1817` (dark on yellow)
  
- Action buttons (Copy/Regenerate/Delete): keep hover effect but use `Theme.glassCardHover` for background

### 4.5 PromptInput

Current: `Theme.inputBg` with `Theme.border`, radius 24.

New:
- Outer container: `GlassPanel { layer: "top"; cornerRadius: 24 }`
  - But 24px radius is large; keep it
  - Base: `Theme.glassInput` @ `Theme.glassInputOpacity`
  - Border: `Theme.glassBorder` @ `Theme.glassBorderOpacity`
  - Focus: border → `Theme.accent`, plus glow effect
- Send button: keep current accent circle, but when disabled use `Theme.glassSurface` instead of `Theme.border`

### 4.6 EmptyState

Current: transparent background, prompt buttons are solid `Theme.cardBg` with border.

New:
- Main text: keep as is
- Prompt buttons: `GlassPanel { layer: "top"; cornerRadius: 18 }`
  - Hover: `Theme.glassCardHover` @ `Theme.glassCardHoverOpacity`

### 4.7 MessageList

Keep spacing at 16px. The glass bubbles will naturally create visual separation.

---

## 5. Settings Mode Redesign

### 5.1 SettingsSidebar

Current: solid `Theme.surface`.

New:
- Background: `color: Theme.glassSurface; opacity: Theme.glassSurfaceOpacity`
- Right border: 1px `Theme.glassBorder` @ `Theme.glassBorderOpacity`
- Top highlight: subtle light reflection at top

**Group headers**:
- Keep current text styling
- Arrow indicator: use `Theme.textSecondary` with subtle opacity
- Expanded group: background stays transparent, but the selected sub-item gets a glass highlight

**Sub-items**:
- Selected: `GlassPanel { layer: "top"; cornerRadius: 6 }` with `color: Theme.glassCard`, or just a rounded Rectangle with `Theme.glassCardHover`
- Unselected: transparent

### 5.2 SettingsPanel

Current: solid `Theme.bg`.

New:
- Background: transparent (the AppWindow radial gradient shows through)
- Empty state grid cards: `GlassPanel { layer: "top"; cornerRadius: 8 }`
  - Hover: `Theme.glassCardHover` @ `Theme.glassCardHoverOpacity`

### 5.3 ConfigSection (Field Rendering)

Current issues from reference screenshot:
- Field spacing 24px is too large → reduce to **16px**
- Label-to-input spacing 6px → increase to **8px**
- Fields feel "floating" with no container

New:
- Wrap all fields of a section inside a `GlassPanel { layer: "top"; cornerRadius: 12 }`
  - This creates a "glass card" that groups the section's fields
  - Internal padding: 20px
  - Field spacing inside card: 16px
- Section title: keep 22px Bold, but add a subtle bottom divider (1px `Theme.glassBorder`) inside the card
- Section summary: keep 13px `Theme.textSecondary`

**Individual fields**:
- StringField / FloatField / SecretField / TextAreaField: `GlassInput` treatment
  - Background: `Theme.glassInput` @ `Theme.glassInputOpacity`
  - Radius: 8px
  - Border: 1px `Theme.glassBorder` @ `Theme.glassBorderOpacity`
  - Focus: 2px `Theme.accent` + glow
- NumberField: same glass treatment for the contentItem TextField; up/down indicators use `Theme.glassSurface`
- BoolField (Switch): keep current pill design but use glass colors
- EnumField (ComboBox): glass dropdown background
- MultiSelectField: chips use glass treatment — selected chip uses `Theme.accent` @ `0.25` bg + `Theme.accent` border

### 5.4 Editor Components (ProviderEditor, FallbackProviderEditor, etc.)

These are specialized headers + ConfigSection.

- Editor header area: wrap in `GlassPanel { layer: "mid"; cornerRadius: 8 }`
- Action buttons (Fetch Models, Test Connection, Delete): glass-styled buttons
  - Primary action: `Theme.accent` background, `#1a1817` text
  - Secondary/Danger: `GlassPanel` with hover effect

---

## 6. Global Component Changes

### 6.1 Toast

Current: likely solid background.

New: `GlassPanel { layer: "top"; cornerRadius: 8 }` with `Theme.glassCard`.

### 6.2 ScrollToBottomButton

Keep current but use glass colors.

### 6.3 ToolCallCard

Use `GlassPanel { layer: "top"; cornerRadius: 8 }`.

### 6.4 StreamingCursor

Keep as is (it's a small blinking element).

---

## 7. Animation & Interaction

- **Hover transitions**: All glass panels should animate opacity/color changes over 150ms using `Behavior on opacity` and `Behavior on color`
- **Focus glow**: Input fields get a 200ms fade-in of the accent glow Rectangle
- **Message bubble entrance**: Optional subtle fade-in + slight scale-up (0.98 → 1.0) over 200ms for new messages
- **Settings card hover**: 150ms transition to hover state

---

## 8. File Modification List

### C++ Theme
- `desktop/src/Theme.h` — Add glass color and opacity properties
- `desktop/src/Theme.cpp` — Implement property getters with dark/light switch logic

### New QML Components
- `desktop/qml/components/GlassPanel.qml` — Reusable glass panel
- `desktop/qml/components/GlassInput.qml` — Reusable glass input background (optional, could inline)

### Modified QML Files
- `desktop/qml/AppWindow.qml` — Radial gradient background
- `desktop/qml/Shell/TopBar.qml` — Glass surface background
- `desktop/qml/ChatMode/ChatWorkspace.qml` — Transparent background (let gradient through)
- `desktop/qml/ChatMode/ConversationHeader.qml` — Glass surface
- `desktop/qml/ChatMode/MessageBubble.qml` — Glass bubbles (AI neutral, user accent-glass)
- `desktop/qml/ChatMode/MessageList.qml` — Keep structure, spacing may adjust
- `desktop/qml/ChatMode/PromptInput.qml` — Glass input container
- `desktop/qml/ChatMode/EmptyState.qml` — Glass prompt buttons
- `desktop/qml/ChatMode/ToolCallCard.qml` — Glass card
- `desktop/qml/SettingsMode/SettingsSidebar.qml` — Glass sidebar
- `desktop/qml/SettingsMode/SettingsPanel.qml` — Transparent background, glass empty cards
- `desktop/qml/SettingsMode/ConfigSection.qml` — Glass card wrapper, tighter spacing
- `desktop/qml/SettingsMode/fields/StringField.qml` — Glass input
- `desktop/qml/SettingsMode/fields/NumberField.qml` — Glass input + glass indicators
- `desktop/qml/SettingsMode/fields/FloatField.qml` — Glass input
- `desktop/qml/SettingsMode/fields/BoolField.qml` — Glass switch colors
- `desktop/qml/SettingsMode/fields/EnumField.qml` — Glass dropdown
- `desktop/qml/SettingsMode/fields/SecretField.qml` — Glass input
- `desktop/qml/SettingsMode/fields/TextAreaField.qml` — Glass textarea
- `desktop/qml/SettingsMode/fields/MultiSelectField.qml` — Glass chips
- `desktop/qml/SettingsMode/editors/*.qml` — Glass editor headers
- `desktop/qml/components/Toast.qml` — Glass toast

---

## 9. Testing Checklist

- [ ] Dark mode glass panels look correct (not too transparent, readable text)
- [ ] Light mode glass panels look correct (not washed out)
- [ ] Message bubbles have clear visual distinction between user and AI
- [ ] Input focus glow is visible but not distracting
- [ ] Settings field spacing is comfortable (16px between fields, 8px label-to-input)
- [ ] Sidebar selected item is clearly highlighted
- [ ] Empty state grid cards hover correctly
- [ ] No regression in functionality (tabs, buttons, inputs, switches all work)
- [ ] Radial gradient background renders without performance issues

---

## 10. Risk Assessment

| Risk | Mitigation |
|------|------------|
| Glass opacity too low → text unreadable | Start conservative (70-80% opacity), tune per component |
| RadialGradient performance on large window | Test on target hardware; fall back to linear gradient if needed |
| Too many nested Rectangles for border/highlight | Keep nesting shallow (max 2 levels: base + border/highlight) |
| Inconsistent feel between Chat and Settings | Use `GlassPanel` reusable component everywhere |
| Light mode glass looks like plain white | Ensure borders and shadows are visible in light mode |
