#ifndef HERMIND_MEMORY_H
#define HERMIND_MEMORY_H

#include <QString>
#include <QDateTime>
#include <QJsonObject>

class HermindMemory
{
public:
    HermindMemory() = default;
    static HermindMemory fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    int id() const;
    QString scope() const;     // "workspace" | "global"
    QString content() const;
    QDateTime lastUsedAt() const;
    QDateTime createdAt() const;
    QDateTime updatedAt() const;

    void setId(int id);
    void setScope(const QString &scope);
    void setContent(const QString &content);

private:
    int m_id = 0;
    QString m_scope;
    QString m_content;
    QDateTime m_lastUsedAt;
    QDateTime m_createdAt;
    QDateTime m_updatedAt;
};

#endif // HERMIND_MEMORY_H
