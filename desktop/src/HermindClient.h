#ifndef HERMINDCLIENT_H
#define HERMINDCLIENT_H

#include <QObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QJsonObject>
#include <QJSValue>
#include <functional>

class HermindClient : public QObject
{
    Q_OBJECT
public:
    explicit HermindClient(const QString &baseUrl, QObject *parent = nullptr);

    using Callback = std::function<void(const QJsonObject &, const QString &error)>;

    // C++ API (std::function callbacks)
    void get(const QString &path, Callback callback);
    void post(const QString &path, const QJsonObject &body, Callback callback);
    void put(const QString &path, const QJsonObject &body, Callback callback);
    void delete_(const QString &path, Callback callback);
    void upload(const QString &path, const QByteArray &data,
                const QString &fileName, const QString &mimeType,
                Callback callback);

    // QML API (QJSValue callbacks)
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
