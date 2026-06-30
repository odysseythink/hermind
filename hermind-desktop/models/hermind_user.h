#ifndef HERMIND_USER_H
#define HERMIND_USER_H

#include <QString>
#include <QJsonObject>
#include <optional>

class HermindUser
{
public:
    HermindUser() = default;

    static HermindUser fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    int id() const;
    QString username() const;
    QString role() const;
    int suspended() const;
    std::optional<QString> pfpFilename() const;

private:
    int m_id = 0;
    QString m_username;
    QString m_role;
    int m_suspended = 0;
    std::optional<QString> m_pfpFilename;
};

#endif // HERMIND_USER_H
