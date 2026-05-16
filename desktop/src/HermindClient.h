#ifndef HERMINDCLIENT_H
#define HERMINDCLIENT_H

#include <QObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QJsonObject>
#include <QJSValue>

class HermindClient : public QObject
{
    Q_OBJECT
public:
    explicit HermindClient(const QString &baseUrl, QObject *parent = nullptr);

    Q_INVOKABLE void get(const QString &path, QJSValue callback);
    Q_INVOKABLE void post(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void put(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void delete_(const QString &path, QJSValue callback);
    Q_INVOKABLE void upload(const QString &path, const QByteArray &data,
                            const QString &fileName, const QString &mimeType,
                            QJSValue callback);
    Q_INVOKABLE QNetworkReply* getStream(const QString &path);
    QString baseUrl() const;

private:
    QNetworkAccessManager *m_manager;
    QString m_baseUrl;
};

#endif
