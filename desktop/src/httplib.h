#ifndef HTTPLIB_H
#define HTTPLIB_H

#include <QObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QJsonObject>
#include <QJsonDocument>
#include <functional>

class HermindClient : public QObject
{
    Q_OBJECT
public:
    explicit HermindClient(const QString &baseUrl, QObject *parent = nullptr);

    using Callback = std::function<void(const QJsonObject &, const QString &error)>;

    void get(const QString &path, Callback callback);
    void post(const QString &path, const QJsonObject &body, Callback callback);
    QNetworkReply* getStream(const QString &path);

    QString baseUrl() const;

private:
    QNetworkAccessManager *m_manager;
    QString m_baseUrl;
};

#endif
