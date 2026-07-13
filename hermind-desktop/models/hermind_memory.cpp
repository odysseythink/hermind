#include "hermind_memory.h"
#include <QJsonObject>

HermindMemory HermindMemory::fromJson(const QJsonObject &obj)
{
    HermindMemory m;
    m.m_id = obj.value(QStringLiteral("id")).toInt();
    m.m_scope = obj.value(QStringLiteral("scope")).toString();
    m.m_content = obj.value(QStringLiteral("content")).toString();

    const QJsonValue lu = obj.value(QStringLiteral("lastUsedAt"));
    if (!lu.isNull())
        m.m_lastUsedAt = QDateTime::fromString(lu.toString(), Qt::ISODate);

    const QJsonValue ca = obj.value(QStringLiteral("createdAt"));
    if (!ca.isNull())
        m.m_createdAt = QDateTime::fromString(ca.toString(), Qt::ISODate);

    const QJsonValue ua = obj.value(QStringLiteral("updatedAt"));
    if (!ua.isNull())
        m.m_updatedAt = QDateTime::fromString(ua.toString(), Qt::ISODate);

    return m;
}

QJsonObject HermindMemory::toJson() const
{
    QJsonObject obj;
    obj.insert(QStringLiteral("id"), m_id);
    obj.insert(QStringLiteral("scope"), m_scope);
    obj.insert(QStringLiteral("content"), m_content);
    if (m_lastUsedAt.isValid())
        obj.insert(QStringLiteral("lastUsedAt"), m_lastUsedAt.toString(Qt::ISODate));
    if (m_createdAt.isValid())
        obj.insert(QStringLiteral("createdAt"), m_createdAt.toString(Qt::ISODate));
    if (m_updatedAt.isValid())
        obj.insert(QStringLiteral("updatedAt"), m_updatedAt.toString(Qt::ISODate));
    return obj;
}

int HermindMemory::id() const { return m_id; }
QString HermindMemory::scope() const { return m_scope; }
QString HermindMemory::content() const { return m_content; }
QDateTime HermindMemory::lastUsedAt() const { return m_lastUsedAt; }
QDateTime HermindMemory::createdAt() const { return m_createdAt; }
QDateTime HermindMemory::updatedAt() const { return m_updatedAt; }

void HermindMemory::setId(int id) { m_id = id; }
void HermindMemory::setScope(const QString &s) { m_scope = s; }
void HermindMemory::setContent(const QString &c) { m_content = c; }
