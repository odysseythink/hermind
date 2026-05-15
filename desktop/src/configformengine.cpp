#include "configformengine.h"

#include <QFormLayout>
#include <QLineEdit>
#include <QComboBox>
#include <QCheckBox>
#include <QSpinBox>
#include <QDoubleSpinBox>
#include <QLabel>
#include <QJsonArray>

ConfigFormEngine::ConfigFormEngine(QWidget *parent)
    : QWidget(parent),
      m_layout(new QFormLayout(this)),
      m_isDirty(false)
{
    m_layout->setContentsMargins(16, 16, 16, 16);
    m_layout->setSpacing(12);
    m_layout->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);
}

void ConfigFormEngine::buildForm(const QJsonObject &schema, const QJsonObject &values)
{
    m_schema = schema;
    m_originalValues = values;
    m_isDirty = false;

    // Clear existing editors
    for (auto it = m_editors.begin(); it != m_editors.end(); ++it) {
        delete it.value();
    }
    m_editors.clear();

    // Remove all rows from form layout
    while (m_layout->rowCount() > 0) {
        m_layout->removeRow(0);
    }

    for (auto it = schema.begin(); it != schema.end(); ++it) {
        QString key = it.key();
        QJsonObject fieldSchema = it.value().toObject();

        QWidget *editor = createEditor(key, fieldSchema);
        if (!editor)
            continue;

        m_editors.insert(key, editor);
        QString label = fieldSchema.value(QStringLiteral("title")).toString();
        if (label.isEmpty())
            label = key;
        m_layout->addRow(label + QLatin1Char(':'), editor);

        if (values.contains(key)) {
            setEditorValue(editor, values.value(key));
        } else if (fieldSchema.contains(QStringLiteral("default"))) {
            setEditorValue(editor, fieldSchema.value(QStringLiteral("default")));
        }

        trackEditor(editor, key);
    }
}

QWidget* ConfigFormEngine::createEditor(const QString &key, const QJsonObject &fieldSchema)
{
    Q_UNUSED(key)
    QString type = fieldSchema.value(QStringLiteral("type")).toString();
    QJsonArray enumValues = fieldSchema.value(QStringLiteral("enum")).toArray();

    if (!enumValues.isEmpty()) {
        QComboBox *cb = new QComboBox(this);
        for (const auto &v : enumValues) {
            cb->addItem(v.toString());
        }
        return cb;
    }

    if (type == QStringLiteral("boolean")) {
        QCheckBox *chk = new QCheckBox(this);
        return chk;
    }

    if (type == QStringLiteral("integer")) {
        QSpinBox *sb = new QSpinBox(this);
        if (fieldSchema.contains(QStringLiteral("minimum")))
            sb->setMinimum(fieldSchema.value(QStringLiteral("minimum")).toInt());
        if (fieldSchema.contains(QStringLiteral("maximum")))
            sb->setMaximum(fieldSchema.value(QStringLiteral("maximum")).toInt());
        if (fieldSchema.contains(QStringLiteral("default")))
            sb->setValue(fieldSchema.value(QStringLiteral("default")).toInt());
        return sb;
    }

    if (type == QStringLiteral("number")) {
        QDoubleSpinBox *dsb = new QDoubleSpinBox(this);
        dsb->setDecimals(2);
        if (fieldSchema.contains(QStringLiteral("minimum")))
            dsb->setMinimum(fieldSchema.value(QStringLiteral("minimum")).toDouble());
        if (fieldSchema.contains(QStringLiteral("maximum")))
            dsb->setMaximum(fieldSchema.value(QStringLiteral("maximum")).toDouble());
        if (fieldSchema.contains(QStringLiteral("default")))
            dsb->setValue(fieldSchema.value(QStringLiteral("default")).toDouble());
        return dsb;
    }

    if (type == QStringLiteral("array")) {
        QLineEdit *le = new QLineEdit(this);
        le->setPlaceholderText(QStringLiteral("Comma-separated values"));
        return le;
    }

    // Default: string
    QLineEdit *le = new QLineEdit(this);
    if (fieldSchema.value(QStringLiteral("format")).toString() == QStringLiteral("password")) {
        le->setEchoMode(QLineEdit::Password);
    }
    return le;
}

void ConfigFormEngine::trackEditor(QWidget *editor, const QString &key)
{
    Q_UNUSED(key)

    if (QLineEdit *le = qobject_cast<QLineEdit*>(editor)) {
        connect(le, &QLineEdit::textChanged, this, &ConfigFormEngine::checkDirty);
    } else if (QComboBox *cb = qobject_cast<QComboBox*>(editor)) {
        connect(cb, QOverload<int>::of(&QComboBox::currentIndexChanged), this, &ConfigFormEngine::checkDirty);
    } else if (QCheckBox *chk = qobject_cast<QCheckBox*>(editor)) {
        connect(chk, &QCheckBox::checkStateChanged, this, [this](Qt::CheckState) { checkDirty(); });
    } else if (QSpinBox *sb = qobject_cast<QSpinBox*>(editor)) {
        connect(sb, QOverload<int>::of(&QSpinBox::valueChanged), this, &ConfigFormEngine::checkDirty);
    } else if (QDoubleSpinBox *dsb = qobject_cast<QDoubleSpinBox*>(editor)) {
        connect(dsb, QOverload<double>::of(&QDoubleSpinBox::valueChanged), this, &ConfigFormEngine::checkDirty);
    }
}

QJsonValue ConfigFormEngine::getEditorValue(QWidget *editor) const
{
    if (QLineEdit *le = qobject_cast<QLineEdit*>(editor)) {
        return le->text();
    }
    if (QComboBox *cb = qobject_cast<QComboBox*>(editor)) {
        return cb->currentText();
    }
    if (QCheckBox *chk = qobject_cast<QCheckBox*>(editor)) {
        return chk->isChecked();
    }
    if (QSpinBox *sb = qobject_cast<QSpinBox*>(editor)) {
        return sb->value();
    }
    if (QDoubleSpinBox *dsb = qobject_cast<QDoubleSpinBox*>(editor)) {
        return dsb->value();
    }
    return QJsonValue();
}

void ConfigFormEngine::setEditorValue(QWidget *editor, const QJsonValue &value)
{
    if (QLineEdit *le = qobject_cast<QLineEdit*>(editor)) {
        le->setText(value.toString());
    } else if (QComboBox *cb = qobject_cast<QComboBox*>(editor)) {
        int idx = cb->findText(value.toString());
        if (idx >= 0)
            cb->setCurrentIndex(idx);
    } else if (QCheckBox *chk = qobject_cast<QCheckBox*>(editor)) {
        chk->setChecked(value.toBool());
    } else if (QSpinBox *sb = qobject_cast<QSpinBox*>(editor)) {
        sb->setValue(value.toInt());
    } else if (QDoubleSpinBox *dsb = qobject_cast<QDoubleSpinBox*>(editor)) {
        dsb->setValue(value.toDouble());
    }
}

void ConfigFormEngine::checkDirty()
{
    bool dirty = false;
    for (auto it = m_editors.begin(); it != m_editors.end(); ++it) {
        QString key = it.key();
        QJsonValue current = getEditorValue(it.value());
        QJsonValue original = m_originalValues.value(key);

        if (current != original) {
            dirty = true;
            break;
        }
    }

    if (dirty != m_isDirty) {
        m_isDirty = dirty;
        emit dirtyChanged(dirty);
    }
}

QJsonObject ConfigFormEngine::values() const
{
    QJsonObject result;
    for (auto it = m_editors.begin(); it != m_editors.end(); ++it) {
        QJsonValue v = getEditorValue(it.value());
        if (!v.isUndefined()) {
            result[it.key()] = v;
        }
    }
    return result;
}

void ConfigFormEngine::setValues(const QJsonObject &values)
{
    for (auto it = values.begin(); it != values.end(); ++it) {
        QWidget *editor = m_editors.value(it.key());
        if (editor) {
            setEditorValue(editor, it.value());
        }
    }
    m_originalValues = values;
    m_isDirty = false;
}

bool ConfigFormEngine::isDirty() const
{
    return m_isDirty;
}
