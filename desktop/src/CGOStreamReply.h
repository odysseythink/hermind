#ifndef CGOSTREAMREPLY_H
#define CGOSTREAMREPLY_H

#include <QNetworkReply>
#include <QBuffer>

class CGOStreamReply : public QNetworkReply
{
    Q_OBJECT
public:
    explicit CGOStreamReply(const QString &path, QObject *parent = nullptr);

    void appendChunk(const QByteArray &data);
    void finish();
    void setStreamError(const QString &error);

    qint64 bytesAvailable() const override;
    void abort() override;

protected:
    qint64 readData(char *data, qint64 maxlen) override;

private:
    QByteArray m_buffer;
    qint64 m_readPos = 0;
    bool m_finished = false;
};

#endif
