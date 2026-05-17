#include "CGOStreamReply.h"
#include <QMetaObject>
#include <QJsonDocument>
#include <QJsonObject>
#include <QMap>

// Global map of active stream replies by path
static QMap<QString, CGOStreamReply*> g_streamReplies;

extern "C" {

// This function is called by Go via CGO
void C_StreamCallback(const char* eventType, const char* data)
{
    QString typeStr = QString::fromUtf8(eventType);
    QString dataStr = QString::fromUtf8(data);

    // For MVP, broadcast to all active stream replies
    // In production, match by stream ID
    for (CGOStreamReply *reply : g_streamReplies) {
        if (typeStr == QStringLiteral("chunk")) {
            reply->appendChunk(dataStr.toUtf8());
        } else if (typeStr == QStringLiteral("done")) {
            reply->finish();
        } else if (typeStr == QStringLiteral("error")) {
            reply->setStreamError(dataStr);
        }
    }
}

} // extern "C"

// Helpers to register/unregister stream replies
void registerStreamReply(const QString &path, CGOStreamReply *reply)
{
    g_streamReplies[path] = reply;
}

void unregisterStreamReply(const QString &path)
{
    g_streamReplies.remove(path);
}
