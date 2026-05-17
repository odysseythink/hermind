import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

GlassPanel {
    property var section
    property var value
    property var originalValue
    property var config
    signal fieldChanged(string name, var value)

    baseColor: Theme.glassCard
    highlightStrength: 0.1

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 14

        Text {
            text: section.label || section.key
            font.pixelSize: 22
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
            visible: section.summary !== undefined && section.summary.length > 0
        }

        Rectangle {
            Layout.fillWidth: true
            height: 1
            color: Theme.glassBorder
            visible: section.summary !== undefined && section.summary.length > 0
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

                Component {
                    id: stringComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        StringField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v) }
                        }
                    }
                }
                Component {
                    id: numberComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        NumberField {
                            field: modelData
                            fieldValue: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, parseInt(v)) }
                        }
                    }
                }
                Component {
                    id: floatComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        FloatField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, parseFloat(v)) }
                        }
                    }
                }
                Component {
                    id: boolComp
                    RowLayout {
                        BoolField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v === "true") }
                        }
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                    }
                }
                Component {
                    id: enumComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        EnumField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v) }
                        }
                    }
                }
                Component {
                    id: secretComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        SecretField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v) }
                        }
                    }
                }
                Component {
                    id: textAreaComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        TextAreaField {
                            field: modelData
                            value: getFieldValue(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v) }
                        }
                    }
                }
                Component {
                    id: multiSelectComp
                    ColumnLayout {
                        spacing: 8
                        Text { text: modelData.label || modelData.name; color: Theme.textPrimary; font.pixelSize: 13 }
                        MultiSelectField {
                            field: modelData
                            value: getFieldValueArray(value, modelData.name)
                            onChanged: function(v) { fieldChanged(modelData.name, v) }
                        }
                    }
                }
            }
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
