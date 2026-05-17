#ifndef HERMINDCGOCLIENT_H
#define HERMINDCGOCLIENT_H

#include <QObject>
#include <QJsonObject>
#include <QJsonArray>
#include <QJSValue>
#include <QNetworkReply>
#include <functional>

class HermindCGOClient : public QObject
{
    Q_OBJECT
public:
    explicit HermindCGOClient(QObject *parent = nullptr);

    using Callback = std::function<void(const QJsonObject &, const QString &error)>;

    // C++ API
    void get(const QString &path, Callback callback);
    void post(const QString &path, const QJsonObject &body, Callback callback);
    void put(const QString &path, const QJsonObject &body, Callback callback);
    void delete_(const QString &path, Callback callback);
    void upload(const QString &path, const QByteArray &data,
                const QString &fileName, const QString &mimeType,
                Callback callback);
    QNetworkReply* getStream(const QString &path);

    // QML API
    Q_INVOKABLE void get(const QString &path, QJSValue callback);
    Q_INVOKABLE void post(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void put(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void delete_(const QString &path, QJSValue callback);
    Q_INVOKABLE void upload(const QString &path, const QByteArray &data,
                            const QString &fileName, const QString &mimeType,
                            QJSValue callback);
    QString baseUrl() const { return QStringLiteral("cgo://internal"); }

private:
    static QJsonObject doCall(const QString &method, const QString &path,
                               const QJsonObject &body = QJsonObject());
    static void invokeJSCallback(QJSValue callback, const QJsonObject &resp, const QString &error);
};

#endif
