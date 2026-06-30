#include "hermind_user.h"

HermindUser HermindUser::fromJson(const QJsonObject &obj)
{
    HermindUser user;
    user.m_id = obj.value("id").toInt();
    user.m_username = obj.value("username").toString();
    user.m_role = obj.value("role").toString();
    user.m_suspended = obj.value("suspended").toInt();

    if (obj.contains("pfpFilename") && !obj.value("pfpFilename").isNull()) {
        user.m_pfpFilename = obj.value("pfpFilename").toString();
    }
    return user;
}

QJsonObject HermindUser::toJson() const
{
    QJsonObject obj;
    obj.insert("id", m_id);
    obj.insert("username", m_username);
    obj.insert("role", m_role);
    obj.insert("suspended", m_suspended);
    if (m_pfpFilename.has_value()) {
        obj.insert("pfpFilename", m_pfpFilename.value());
    } else {
        obj.insert("pfpFilename", QJsonValue::Null);
    }
    return obj;
}

int HermindUser::id() const { return m_id; }
QString HermindUser::username() const { return m_username; }
QString HermindUser::role() const { return m_role; }
int HermindUser::suspended() const { return m_suspended; }
std::optional<QString> HermindUser::pfpFilename() const { return m_pfpFilename; }
