# Glassmorphism UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the entire Hermind desktop QML UI with a layered depth glassmorphism aesthetic — translucent panels with light reflections, subtle borders, and a radial gradient background.

**Architecture:** Extend the C++ `Theme` singleton with glass color properties (using QColor with alpha), create a reusable `GlassPanel.qml` component, then systematically replace solid backgrounds across all QML files. Background becomes a radial gradient; panels become translucent with top-edge highlights; inputs get focus glow.

**Tech Stack:** Qt 6.10, QML, C++ (no new modules — pure QML gradients and opacity)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `desktop/src/Theme.h` | Modify | Add glass color properties with alpha-channel QColor |
| `desktop/qml/components/GlassPanel.qml` | Create | Reusable glass panel with top highlight + border |
| `desktop/qml/AppWindow.qml` | Modify | Radial gradient background |
| `desktop/qml/Shell/TopBar.qml` | Modify | Glass surface background |
| `desktop/qml/ChatMode/ChatWorkspace.qml` | Modify | Transparent background |
| `desktop/qml/ChatMode/ConversationHeader.qml` | Modify | Glass surface background |
| `desktop/qml/ChatMode/MessageBubble.qml` | Modify | AI = neutral glass; User = accent-tinted glass |
| `desktop/qml/ChatMode/PromptInput.qml` | Modify | Glass input container with focus glow |
| `desktop/qml/ChatMode/EmptyState.qml` | Modify | Glass prompt buttons |
| `desktop/qml/ChatMode/ToolCallCard.qml` | Modify | Glass card |
| `desktop/qml/SettingsMode/SettingsSidebar.qml` | Modify | Glass sidebar + glass selected item |
| `desktop/qml/SettingsMode/SettingsPanel.qml` | Modify | Transparent bg; glass empty-state cards |
| `desktop/qml/SettingsMode/ConfigSection.qml` | Modify | Glass card wrapper; tighter spacing |
| `desktop/qml/SettingsMode/fields/StringField.qml` | Modify | Glass input background |
| `desktop/qml/SettingsMode/fields/NumberField.qml` | Modify | Glass input + glass indicators |
| `desktop/qml/SettingsMode/fields/FloatField.qml` | Modify | Glass input background |
| `desktop/qml/SettingsMode/fields/BoolField.qml` | Modify | Glass switch colors |
| `desktop/qml/SettingsMode/fields/EnumField.qml` | Modify | Glass dropdown background |
| `desktop/qml/SettingsMode/fields/SecretField.qml` | Modify | Glass input background |
| `desktop/qml/SettingsMode/fields/TextAreaField.qml` | Modify | Glass textarea background |
| `desktop/qml/SettingsMode/fields/MultiSelectField.qml` | Modify | Glass chips |
| `desktop/qml/SettingsMode/editors/*.qml` | Modify (7 files) | Glass editor headers |
| `desktop/qml/components/Toast.qml` | Modify | Glass toast background |

---

## Implementation Notes

**QColor with Alpha:** All glass colors are exposed as `QColor` with built-in alpha (e.g., `QColor(26, 28, 33, 178)`). In QML, `Rectangle { color: Theme.glassCard }` renders translucently **without** affecting child text opacity. This is critical — never use `Rectangle.opacity` for glass effects.

**GlassPanel.qml pattern:** A reusable component that combines:
1. Base translucent color
2. A thin top highlight Rectangle (simulates light reflection on glass)
3. A border Rectangle (transparent fill, colored border)

**Focus glow pattern:** For inputs, place a slightly larger `Rectangle` behind the input with `color: Theme.glassGlow`, visible only on `activeFocus`.

---

## Task 1: Extend Theme.h with Glass Colors

**Files:**
- Modify: `desktop/src/Theme.h`

**Rationale:** All glass colors need dark/light variants with alpha. Using QColor with alpha avoids opacity inheritance issues in QML.

- [ ] **Step 1: Add glass property declarations to Theme.h**

Add these `Q_PROPERTY` entries after the existing `inputBg` property (around line 24):

```cpp
    Q_PROPERTY(QColor glassSurface READ glassSurface NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassSurfaceHover READ glassSurfaceHover NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassCard READ glassCard NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassCardHover READ glassCardHover NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassInput READ glassInput NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassBorder READ glassBorder NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassGlow READ glassGlow NOTIFY isDarkChanged)
    Q_PROPERTY(QColor glassShadow READ glassShadow NOTIFY isDarkChanged)
    Q_PROPERTY(QColor bgGradientCenter READ bgGradientCenter NOTIFY isDarkChanged)
    Q_PROPERTY(QColor bgGradientEdge READ bgGradientEdge NOTIFY isDarkChanged)
```

- [ ] **Step 2: Add getter implementations in Theme.h**

Add these public getter methods before the `signals:` section (around line 46):

```cpp
    QColor glassSurface() const { return m_isDark ? QColor(20, 22, 26, 217) : QColor(240, 237, 228, 217); }
    QColor glassSurfaceHover() const { return m_isDark ? QColor(29, 32, 39, 230) : QColor(232, 228, 214, 230); }
    QColor glassCard() const { return m_isDark ? QColor(26, 28, 33, 178) : QColor(245, 243, 237, 178); }
    QColor glassCardHover() const { return m_isDark ? QColor(29, 32, 39, 204) : QColor(232, 228, 214, 204); }
    QColor glassInput() const { return m_isDark ? QColor(20, 22, 26, 191) : QColor(255, 255, 255, 204); }
    QColor glassBorder() const { return m_isDark ? QColor(42, 46, 54, 128) : QColor(217, 212, 200, 102); }
    QColor glassGlow() const { return m_isDark ? QColor(255, 184, 0, 77) : QColor(255, 184, 0, 64); }
    QColor glassShadow() const { return m_isDark ? QColor(0, 0, 0, 51) : QColor(0, 0, 0, 26); }
    QColor bgGradientCenter() const { return m_isDark ? QColor(13, 17, 23) : QColor(250, 250, 247); }
    QColor bgGradientEdge() const { return m_isDark ? QColor(10, 11, 13) : QColor(240, 237, 228); }
```

- [ ] **Step 3: Verify build compiles**

Run: `cd D:/go_work/hermind/desktop/build/Debug && cmake --build . --target hermind-desktop 2>&1 | tail -20`

Expected: No C++ compilation errors. Link may fail if exe is locked — that's OK, we just need the obj files to compile.

---

## Task 2: Create Reusable GlassPanel Component

**Files:**
- Create: `desktop/qml/components/GlassPanel.qml`

**Rationale:** Every glass surface uses the same pattern (base color + top highlight + border). A reusable component ensures consistency and makes future tweaks trivial.

- [ ] **Step 1: Create the file**

```qml
import QtQuick

Rectangle {
    id: root

    // Configurable properties
    property color baseColor: Theme.glassCard
    property color highlightColor: Theme.glassSurfaceHover
    property color borderColor: Theme.glassBorder
    property real highlightStrength: 0.12
    property real borderOpacity: 1.0

    // Main glass background
    color: baseColor
    radius: 8

    // Top highlight — simulates light reflecting off glass surface
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: Math.max(1, parent.radius * 0.6)
        radius: parent.radius
        color: highlightColor
        opacity: highlightStrength

        // Mask bottom half so highlight only appears at top edge
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: root.color
            opacity: 1.0
        }
    }

    // Border overlay (separate so it doesn't get clipped by highlight mask)
    Rectangle {
        anchors.fill: parent
        color: "transparent"
        radius: parent.radius
        border.color: borderColor
        border.width: 1
        opacity: borderOpacity
    }
}
```

- [ ] **Step 2: Register in CMakeLists.txt**

Add `qml/components/GlassPanel.qml` to the `QML_FILES` list in `desktop/CMakeLists.txt` (after the existing `Toast.qml` line around line 97):

```cmake
        qml/components/GlassPanel.qml
```

---

## Task 3: AppWindow Radial Gradient Background

**Files:**
- Modify: `desktop/qml/AppWindow.qml`

- [ ] **Step 1: Replace solid background with radial gradient**

Replace the root `Rectangle` in `AppWindow.qml`:

```qml
Rectangle {
    // Radial gradient background for depth
    gradient: RadialGradient {
        centerX: parent.width / 2
        centerY: parent.height / 2
        centerRadius: Math.max(parent.width, parent.height)
        focalX: centerX
        focalY: centerY
        GradientStop { position: 0; color: Theme.bgGradientCenter }
        GradientStop { position: 1; color: Theme.bgGradientEdge }
    }

    ColumnLayout {
        // ... keep existing contents unchanged ...
    }
}
```

---

## Task 4: TopBar Glass Surface

**Files:**
- Modify: `desktop/qml/Shell/TopBar.qml`

- [ ] **Step 1: Replace TopBar solid background with glass**

Find the root `Rectangle` of TopBar. Replace `color: Theme.surface` and the bottom border Rectangle with:

```qml
Rectangle {
    // Glass surface background
    color: Theme.glassSurface

    // Top subtle highlight
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.glassSurfaceHover
        opacity: 0.3
    }

    // Bottom border
    Rectangle {
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.glassBorder
    }

    // ... existing RowLayout content stays unchanged ...
}
```

Remove the old separate `Rectangle` that was drawing the bottom border (if it exists as a child).

---

## Task 5: ChatWorkspace + ConversationHeader

**Files:**
- Modify: `desktop/qml/ChatMode/ChatWorkspace.qml`
- Modify: `desktop/qml/ChatMode/ConversationHeader.qml`

- [ ] **Step 1: ChatWorkspace — transparent background**

Replace `color: Theme.bg` with `color: "transparent"` so the radial gradient shows through.

```qml
Rectangle {
    color: "transparent"
    // ... rest unchanged
}
```

- [ ] **Step 2: ConversationHeader — glass background**

Replace `color: Theme.surface` with `color: Theme.glassSurface`. Remove the bottom border Rectangle (or change its color to `Theme.glassBorder`).

---

## Task 6: MessageBubble Glass Redesign

**Files:**
- Modify: `desktop/qml/ChatMode/MessageBubble.qml`

- [ ] **Step 1: AI messages — neutral glass**

Replace the AI message styling (when `!isUser`):

```qml
Rectangle {
    property bool isUser: false
    // ... other properties unchanged ...

    // AI messages: neutral glass
    // User messages: accent-tinted glass
    color: isUser ? Qt.rgba(255, 184, 0, 0.25) : Theme.glassCard
    border.color: isUser ? Qt.rgba(255, 184, 0, 0.4) : Theme.glassBorder
    border.width: 1
    radius: 12

    // Top highlight for glass effect
    Rectangle {
        visible: !isUser
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 7
        radius: parent.radius
        color: Theme.glassSurfaceHover
        opacity: 0.15
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: parent.parent.color
            opacity: 1.0
        }
    }

    // User message top highlight (accent tinted)
    Rectangle {
        visible: isUser
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 7
        radius: parent.radius
        color: "#FFE066"
        opacity: 0.3
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: parent.parent.color
            opacity: 1.0
        }
    }

    // ... rest of ColumnLayout unchanged ...
}
```

- [ ] **Step 2: Update role label badge for glass context**

The role label badge (YOU/HERMIND) should use slightly adjusted backgrounds:

For AI badge (around line 31):
```qml
color: Theme.glassSurface
```

For user badge, keep `#1a1817`.

- [ ] **Step 3: Update action button hover**

Action buttons (Copy/Regenerate/Delete) hover color:
```qml
color: actionMouse.containsMouse ? Theme.glassCardHover : "transparent"
```

---

## Task 7: PromptInput Glass Container

**Files:**
- Modify: `desktop/qml/ChatMode/PromptInput.qml`

- [ ] **Step 1: Replace input container with glass**

The outer `Rectangle` (currently `color: Theme.inputBg`, `border.color: Theme.border`) becomes:

```qml
Rectangle {
    Layout.fillWidth: true
    Layout.fillHeight: true
    radius: 24
    color: Theme.glassInput

    // Top highlight
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 10
        radius: parent.radius
        color: Theme.glassSurfaceHover
        opacity: 0.12
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
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
    }

    // Focus glow (behind everything)
    Rectangle {
        id: focusGlow
        anchors.fill: parent
        anchors.margins: -2
        radius: parent.radius + 2
        color: Theme.glassGlow
        opacity: inputArea.activeFocus ? 0.6 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 200 } }
    }

    RowLayout {
        // ... existing content unchanged ...
    }
}
```

- [ ] **Step 2: Update send button disabled state**

Send button when disabled: change `Theme.border` to `Theme.glassSurface`.

---

## Task 8: EmptyState Glass Prompt Buttons

**Files:**
- Modify: `desktop/qml/ChatMode/EmptyState.qml`

- [ ] **Step 1: Replace prompt button rectangles with GlassPanel**

Each prompt button currently is a `Rectangle { color: Theme.cardBg; border.color: Theme.border }`. Replace with:

```qml
GlassPanel {
    Layout.preferredWidth: promptBtnText.implicitWidth + 24
    Layout.preferredHeight: 36
    baseColor: Theme.glassCard
    highlightStrength: 0.1

    Text {
        id: promptBtnText
        anchors.centerIn: parent
        text: modelData
        font.pixelSize: 13
        color: Theme.textSecondary
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        onEntered: parent.baseColor = Theme.glassCardHover
        onExited: parent.baseColor = Theme.glassCard
        onClicked: appState.sendMessage(modelData)
    }
}
```

Apply this pattern to all prompt buttons (both Repeater delegates and the standalone button).

---

## Task 9: ToolCallCard Glass

**Files:**
- Modify: `desktop/qml/ChatMode/ToolCallCard.qml`

- [ ] **Step 1: Replace solid background with glass**

```qml
Rectangle {
    color: Theme.glassCard
    border.color: Theme.glassBorder
    border.width: 1
    radius: 8

    // Top highlight
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 5
        radius: parent.radius
        color: Theme.glassSurfaceHover
        opacity: 0.12
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: parent.parent.color
            opacity: 1.0
        }
    }

    // ... existing RowLayout unchanged ...
}
```

---

## Task 10: SettingsSidebar Glass

**Files:**
- Modify: `desktop/qml/SettingsMode/SettingsSidebar.qml`

- [ ] **Step 1: Replace sidebar background**

Change root `Rectangle` color from `Theme.surface` to `Theme.glassSurface`.

Add right border:
```qml
Rectangle {
    anchors.right: parent.right
    anchors.top: parent.top
    anchors.bottom: parent.bottom
    width: 1
    color: Theme.glassBorder
}
```

- [ ] **Step 2: Glass selected sub-item**

Find the sub-item delegate (the item shown when a group is expanded and a section is selected). Change its background from `Theme.cardBg` to:

```qml
GlassPanel {
    anchors.fill: parent
    baseColor: Theme.glassCard
    highlightStrength: 0.08
    radius: 6
}
```

Or if it's inline, replace the `Rectangle` with `color: Theme.glassCard` and add a border:
```qml
Rectangle {
    color: Theme.glassCard
    border.color: Theme.glassBorder
    border.width: 1
    radius: 6
}
```

- [ ] **Step 3: Group header hover**

Group header hover background: change from `Theme.surfaceHover` to `Theme.glassSurfaceHover`.

---

## Task 11: SettingsPanel Transparent Background + Empty Cards

**Files:**
- Modify: `desktop/qml/SettingsMode/SettingsPanel.qml`

- [ ] **Step 1: Transparent panel background**

Change root `Rectangle` color from `Theme.bg` to `"transparent"`.

- [ ] **Step 2: Glass empty-state cards**

Find the empty state card delegate. Replace its `Rectangle`:

```qml
GlassPanel {
    width: 240
    height: 100
    baseColor: Theme.glassCard
    highlightStrength: 0.1

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        onEntered: parent.baseColor = Theme.glassCardHover
        onExited: parent.baseColor = Theme.glassCard
        // ... onClicked unchanged
    }

    // ... existing Text children unchanged ...
}
```

---

## Task 12: ConfigSection Glass Card Wrapper + Spacing

**Files:**
- Modify: `desktop/qml/SettingsMode/ConfigSection.qml`

- [ ] **Step 1: Wrap content in glass card**

The `ColumnLayout` root should be wrapped or replaced so the entire section sits inside a glass card. The cleanest approach: make the root a `GlassPanel` containing the `ColumnLayout`.

Change root from `ColumnLayout` to:

```qml
GlassPanel {
    baseColor: Theme.glassCard
    highlightStrength: 0.1

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16  // was 24

        // Section title
        Text {
            text: section.label
            font.pixelSize: 22
            font.weight: Font.Bold
            color: Theme.textPrimary
        }

        // Section summary
        Text {
            text: section.summary || ""
            font.pixelSize: 13
            color: Theme.textSecondary
            wrapMode: Text.Wrap
            Layout.fillWidth: true
            visible: text.length > 0
        }

        // Divider
        Rectangle {
            Layout.fillWidth: true
            height: 1
            color: Theme.glassBorder
            visible: section.summary && section.summary.length > 0
        }

        // Fields — spacing inside field wrappers changes from 6 to 8
        // ... existing field rendering logic ...
    }
}
```

- [ ] **Step 2: Increase label-to-input spacing**

In each field wrapper `ColumnLayout` (inside the Loader delegates), change `spacing: 6` to `spacing: 8`.

---

## Task 13: StringField + FloatField + SecretField + TextAreaField

**Files:**
- Modify: `desktop/qml/SettingsMode/fields/StringField.qml`
- Modify: `desktop/qml/SettingsMode/fields/FloatField.qml`
- Modify: `desktop/qml/SettingsMode/fields/SecretField.qml`
- Modify: `desktop/qml/SettingsMode/fields/TextAreaField.qml`

**Pattern for all four:** Replace `Theme.inputBg` with `Theme.glassInput`, and `Theme.border` with `Theme.glassBorder`. Add focus glow.

- [ ] **Step 1: StringField**

```qml
TextField {
    // ... properties unchanged ...

    background: Rectangle {
        color: Theme.glassInput
        border.color: parent.activeFocus ? Theme.accent : Theme.glassBorder
        border.width: parent.activeFocus ? 2 : 1
        radius: 8
    }

    // Focus glow
    Rectangle {
        anchors.fill: parent
        anchors.margins: -2
        radius: 10
        color: Theme.glassGlow
        opacity: parent.activeFocus ? 1.0 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 200 } }
        z: -1
    }
}
```

- [ ] **Step 2: FloatField**

Same pattern as StringField — replace inputBg with glassInput, border with glassBorder, add focus glow.

- [ ] **Step 3: SecretField**

The TextField inside SecretField gets the same glass treatment. The "Show" checkbox area can stay as-is (it's small and functional).

- [ ] **Step 4: TextAreaField**

Same glass background and focus treatment.

---

## Task 14: NumberField Glass SpinBox

**Files:**
- Modify: `desktop/qml/SettingsMode/fields/NumberField.qml`

- [ ] **Step 1: Glass input background**

The `contentItem` TextField gets glass background:
```qml
contentItem: TextField {
    // ... existing properties ...
    background: Rectangle {
        color: Theme.glassInput
        border.color: parent.activeFocus ? Theme.accent : Theme.glassBorder
        border.width: parent.activeFocus ? 2 : 1
        radius: 8
    }
}
```

- [ ] **Step 2: Glass up/down indicators**

Replace indicator backgrounds:
```qml
up.indicator: Rectangle {
    width: 28
    color: Theme.glassSurface
    border.color: Theme.glassBorder
    border.width: 1
    radius: 8
    // ... existing Text child ...
}
```

Same for `down.indicator`.

---

## Task 15: BoolField Glass Switch

**Files:**
- Modify: `desktop/qml/SettingsMode/fields/BoolField.qml`

- [ ] **Step 1: Adjust switch colors for glass context**

The switch indicator pill uses `Theme.border` for off-state and `Theme.accent` for on-state. For glass context, keep these as-is (they're already visually distinct). If the indicator looks out of place against glass backgrounds, no change needed — the accent color pops nicely.

If there is a background Rectangle on the switch, change it to transparent or `Theme.glassInput`.

---

## Task 16: EnumField Glass Dropdown

**Files:**
- Modify: `desktop/qml/SettingsMode/fields/EnumField.qml`

- [ ] **Step 1: Glass ComboBox background**

The ComboBox background and popup should use glass colors:

```qml
background: Rectangle {
    color: Theme.glassInput
    border.color: control.down ? Theme.accent : Theme.glassBorder
    border.width: control.down ? 2 : 1
    radius: 8
}
```

- [ ] **Step 2: Glass popup background**

```qml
popup: Popup {
    // ... existing padding and margins ...
    background: Rectangle {
        color: Theme.glassCard
        border.color: Theme.glassBorder
        border.width: 1
        radius: 8
    }
}
```

---

## Task 17: MultiSelectField Glass Chips

**Files:**
- Modify: `desktop/qml/SettingsMode/fields/MultiSelectField.qml`

- [ ] **Step 1: Glass chip backgrounds**

Selected chips: change solid `Theme.accent` background to a tinted glass:
```qml
// Selected chip
color: Qt.rgba(255, 184, 0, 0.25)
border.color: Theme.accent
border.width: 1
```

Unselected chips: change to `Theme.glassCard` with `Theme.glassBorder`.

---

## Task 18: Settings Editors Glass Headers

**Files:**
- Modify: `desktop/qml/SettingsMode/editors/ProviderEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/FallbackProviderEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/KeyedInstanceEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/ListElementEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/DefaultModelEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/AuxiliaryEditor.qml`
- Modify: `desktop/qml/SettingsMode/editors/SkillsSection.qml`

**Pattern for all seven:** Wrap the editor header (title + action buttons) in a glass panel.

- [ ] **Step 1: ProviderEditor**

Wrap the title and button row:

```qml
GlassPanel {
    Layout.fillWidth: true
    baseColor: Theme.glassSurface
    highlightStrength: 0.08

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        Text {
            text: instanceKey
            font.pixelSize: 20
            font.weight: Font.Bold
            color: Theme.textPrimary
        }

        RowLayout {
            spacing: 12
            // ... existing buttons unchanged ...
        }
    }
}
```

- [ ] **Step 2: FallbackProviderEditor, KeyedInstanceEditor, ListElementEditor**

Apply the same glass header wrapper pattern.

- [ ] **Step 3: DefaultModelEditor, AuxiliaryEditor, SkillsSection**

Apply the same glass header wrapper pattern (these are simpler, just a title + ConfigSection).

---

## Task 19: Toast Glass Background

**Files:**
- Modify: `desktop/qml/components/Toast.qml`

- [ ] **Step 1: Replace with GlassPanel**

```qml
GlassPanel {
    property string message: ""

    visible: message.length > 0
    baseColor: Theme.glassCard
    highlightStrength: 0.1
    radius: 8
    width: toastText.width + 24
    height: toastText.height + 16

    Text {
        id: toastText
        anchors.centerIn: parent
        text: message
        color: Theme.textPrimary
        font.pixelSize: 13
    }
}
```

---

## Task 20: Build and Visual Validation

- [ ] **Step 1: Kill any running hermind-desktop.exe**

```bash
taskkill /F /IM hermind-desktop.exe 2>/dev/null || true
```

- [ ] **Step 2: Build the project**

```bash
cd D:/go_work/hermind/desktop/build/Debug && cmake --build . --target hermind-desktop 2>&1 | tail -30
```

Expected: Link succeeds (exe is no longer locked).

- [ ] **Step 3: Visual check checklist**

Launch the app and verify:
- [ ] Window background shows radial gradient (subtle center-to-edge color shift)
- [ ] TopBar is translucent with visible content behind it
- [ ] AI message bubbles are neutral glass with border and top highlight
- [ ] User message bubbles are golden-tinted glass
- [ ] Text inside all bubbles remains fully opaque and readable
- [ ] PromptInput has glass container; focus shows yellow glow
- [ ] Settings sidebar is translucent glass
- [ ] Settings field sections are wrapped in glass cards
- [ ] Field spacing is tighter (not excessively spaced)
- [ ] Empty state prompt buttons are glass pills
- [ ] Toast appears with glass background
- [ ] Switch to light mode: all glass panels still look correct (not washed out)

- [ ] **Step 4: Tune opacity values if needed**

If any glass panel is too transparent (text hard to read) or too opaque (no glass effect visible), adjust the alpha values in `Theme.h` and rebuild. Typical tuning range:
- `glassCard`: 160-200 alpha (current: 178)
- `glassSurface`: 200-230 alpha (current: 217)
- `glassInput`: 180-220 alpha (current: 191)

- [ ] **Step 5: Commit**

```bash
cd D:/go_work/hermind && git add desktop/src/Theme.h desktop/CMakeLists.txt desktop/qml/components/GlassPanel.qml desktop/qml/
git commit -m "ui: glassmorphism redesign for all interfaces

- Add glass color system to Theme (QColor with alpha)
- Create reusable GlassPanel component
- Radial gradient background for depth
- Glass message bubbles (neutral + accent-tinted)
- Glass input container with focus glow
- Glass settings sidebar, cards, and fields
- Tighter field spacing (24px -> 16px)"
```
