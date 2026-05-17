#include "CGOStreamReply.h"

CGOStreamReply::CGOStreamReply(const QString &path, QObject *parent)
    : QNetworkReply(parent)
{
    setUrl(QUrl(path));
    setOpenMode(QIODevice::ReadOnly);
}

qint64 CGOStreamReply::bytesAvailable() const
{
    return m_buffer.size() - m_readPos + QNetworkReply::bytesAvailable();
}

void CGOStreamReply::abort()
{
}

qint64 CGOStreamReply::readData(char *data, qint64 maxlen)
{
    qint64 len = qMin(maxlen, static_cast<qint64>(m_buffer.size() - m_readPos));
    if (len <= 0) {
        return m_finished ? -1 : 0;
    }
    memcpy(data, m_buffer.constData() + m_readPos, len);
    m_readPos += len;
    return len;
}

void CGOStreamReply::appendChunk(const QByteArray &data)
{
    m_buffer.append(data);
    emit readyRead();
}

void CGOStreamReply::finish()
{
    m_finished = true;
    emit finished();
}

void CGOStreamReply::setStreamError(const QString &error)
{
    setError(UnknownNetworkError, error);
    emit errorOccurred(UnknownNetworkError);
    finish();
}
