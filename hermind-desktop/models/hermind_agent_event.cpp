#include "hermind_agent_event.h"

HermindAgentEvent HermindAgentEvent::fromJson(const QJsonObject &obj)
{
    HermindAgentEvent ev;
    ev.m_type = obj.value("type").toString();
    ev.m_content = obj.value("content").toString();
    ev.m_animate = obj.value("animate").toBool();
    ev.m_question = obj.value("question").toString();
    ev.m_requestId = obj.value("requestId").toString();
    ev.m_skillName = obj.value("skillName").toString();
    ev.m_payload = obj.value("payload");
    ev.m_description = obj.value("description").toString();
    ev.m_timeoutMs = obj.value("timeoutMs").toInt();
    ev.m_from = obj.value("from").toString();
    ev.m_to = obj.value("to").toString();
    ev.m_state = obj.value("state").toString();
    return ev;
}

QJsonObject HermindAgentEvent::toJson() const
{
    QJsonObject obj;
    if (!m_type.isEmpty())
        obj.insert("type", m_type);
    if (!m_content.isEmpty())
        obj.insert("content", m_content);
    if (m_animate)
        obj.insert("animate", m_animate);
    if (!m_question.isEmpty())
        obj.insert("question", m_question);
    if (!m_requestId.isEmpty())
        obj.insert("requestId", m_requestId);
    if (!m_skillName.isEmpty())
        obj.insert("skillName", m_skillName);
    if (!m_payload.isUndefined() && !m_payload.isNull())
        obj.insert("payload", m_payload);
    if (!m_description.isEmpty())
        obj.insert("description", m_description);
    if (m_timeoutMs > 0)
        obj.insert("timeoutMs", m_timeoutMs);
    if (!m_from.isEmpty())
        obj.insert("from", m_from);
    if (!m_to.isEmpty())
        obj.insert("to", m_to);
    if (!m_state.isEmpty())
        obj.insert("state", m_state);
    return obj;
}

QString HermindAgentEvent::type() const { return m_type; }
QString HermindAgentEvent::content() const { return m_content; }
bool HermindAgentEvent::animate() const { return m_animate; }
QString HermindAgentEvent::question() const { return m_question; }
QString HermindAgentEvent::requestId() const { return m_requestId; }
QString HermindAgentEvent::skillName() const { return m_skillName; }
QJsonValue HermindAgentEvent::payload() const { return m_payload; }
QString HermindAgentEvent::description() const { return m_description; }
int HermindAgentEvent::timeoutMs() const { return m_timeoutMs; }
QString HermindAgentEvent::from() const { return m_from; }
QString HermindAgentEvent::to() const { return m_to; }
QString HermindAgentEvent::state() const { return m_state; }
