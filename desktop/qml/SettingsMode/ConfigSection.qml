import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

ColumnLayout {
    property var section
    property var value
    property var originalValue
    property var config
    signal fieldChanged(string name, var value)

    spacing: 16

    Text {
        text: section.label || section.key
        font.pixelSize: 18
        font.weight: Font.Bold
        color: Theme.textPrimary
        visible: section.label !== undefined
    }

    Text {
        text: section.summary || ""
        font.pixelSize: 13
        color: Theme.textSecondary
        wrapMode: Text.Wrap
        Layout.fillWidth: true
        visible: section.summary !== undefined
    }

    Repeater {
        model: section.fields || []
        delegate: Loader {
            id: fieldLoader
            Layout.fillWidth: true
            active: isVisible(modelData, value)

            sourceComponent: {
                switch (modelData.kind) {
                    case "multiselect": return multiSelectComp
                    case "int": return numberComp
                    case "float": return floatComp
                    case "bool": return boolComp
                    case "enum": return enumComp
                    case "secret": return secretComp
                    case "text": return textAreaComp
                    case "string":
                    default: return stringComp
                }
            }

            Component { id: stringComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } StringField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v) } } }
            Component { id: numberComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } NumberField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, parseInt(v)) } } }
            Component { id: floatComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } FloatField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, parseFloat(v)) } } }
            Component { id: boolComp; RowLayout { BoolField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v === "true") } Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } } }
            Component { id: enumComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } EnumField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v) } } }
            Component { id: secretComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } SecretField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v) } } }
            Component { id: textAreaComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } TextAreaField { field: modelData; value: getFieldValue(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v) } } }
            Component { id: multiSelectComp; ColumnLayout { spacing: 4; Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 12 } MultiSelectField { field: modelData; value: getFieldValueArray(value, modelData.name); onChanged: (v) => fieldChanged(modelData.name, v) } } }
        }
    }

    function getFieldValue(obj, fieldName) {
        const v = obj[fieldName]
        return v === undefined || v === null ? "" : String(v)
    }

    function getFieldValueArray(obj, fieldName) {
        const v = obj[fieldName]
        return Array.isArray(v) ? v : []
    }

    function isVisible(field, val) {
        if (!field.visible_when) return true
        const actual = String(val[field.visible_when.field] ?? "")
        if (field.visible_when.in) {
            return field.visible_when.in.some(v => actual === String(v))
        }
        return actual === String(field.visible_when.equals ?? "")
    }
}
