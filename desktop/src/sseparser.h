#ifndef SSEPARSER_H
#define SSEPARSER_H

#include <QObject>
#include <QString>
#include <QByteArray>

class SSEParser : public QObject
{
    Q_OBJECT
public:
    explicit SSEParser(QObject *parent = nullptr);
    void feed(const QByteArray &data);

signals:
    void eventReceived(const QString &eventName, const QString &data);

private:
    QByteArray m_buffer;
    void processBuffer();
};

#endif
