#ifndef CONFIGFORMENGINE_H
#define CONFIGFORMENGINE_H

#include <QWidget>
#include <QJsonObject>
#include <QHash>

class QFormLayout;

class ConfigFormEngine : public QWidget
{
    Q_OBJECT
public:
    explicit ConfigFormEngine(QWidget *parent = nullptr);
    void buildForm(const QJsonObject &schema, const QJsonObject &values = QJsonObject());
    QJsonObject values() const;
    void setValues(const QJsonObject &values);
    bool isDirty() const;

signals:
    void dirtyChanged(bool dirty);

private slots:
    void checkDirty();

private:
    QWidget* createEditor(const QString &key, const QJsonObject &fieldSchema);
    void trackEditor(QWidget *editor, const QString &key);
    QJsonValue getEditorValue(QWidget *editor) const;
    void setEditorValue(QWidget *editor, const QJsonValue &value);

    QFormLayout *m_layout;
    QJsonObject m_schema;
    QJsonObject m_originalValues;
    QHash<QString, QWidget*> m_editors;
    bool m_isDirty;
};

#endif
