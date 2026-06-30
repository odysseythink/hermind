#ifndef HERMIND_AGENT_EVENT_H
#define HERMIND_AGENT_EVENT_H

#include <QString>
#include <QJsonObject>
#include <QJsonValue>

class HermindAgentEvent
{
public:
    HermindAgentEvent() = default;

    static HermindAgentEvent fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    QString type() const;
    QString content() const;
    bool animate() const;
    QString question() const;
    QString requestId() const;
    QString skillName() const;
    QJsonValue payload() const;
    QString description() const;
    int timeoutMs() const;
    QString from() const;
    QString to() const;
    QString state() const;

private:
    QString m_type;
    QString m_content;
    bool m_animate = false;
    QString m_question;
    QString m_requestId;
    QString m_skillName;
    QJsonValue m_payload;
    QString m_description;
    int m_timeoutMs = 0;
    QString m_from;
    QString m_to;
    QString m_state;
};

#endif // HERMIND_AGENT_EVENT_H
