#include "SSEParser.h"

SSEParser::SSEParser(QObject *parent) : QObject(parent) {}

void SSEParser::feed(const QByteArray &data)
{
    m_buffer.append(data);
    processBuffer();
}

void SSEParser::processBuffer()
{
    while (true) {
        int idx = m_buffer.indexOf("\n\n");
        if (idx < 0) break;

        QByteArray block = m_buffer.left(idx);
        m_buffer.remove(0, idx + 2);

        QString eventName = "message";
        QString data;

        for (const QByteArray &line : block.split('\n')) {
            if (line.startsWith("event: ")) {
                eventName = QString::fromUtf8(line.mid(7)).trimmed();
            } else if (line.startsWith("data: ")) {
                if (!data.isEmpty()) data += "\n";
                data += QString::fromUtf8(line.mid(6));
            }
        }

        if (!data.isEmpty()) {
            emit eventReceived(eventName, data);
        }
    }
}
