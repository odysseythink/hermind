#include "httplib.h"
#include <QNetworkRequest>
#include <QUrl>

HermindClient::HermindClient(const QString &baseUrl, QObject *parent)
    : QObject(parent), m_manager(new QNetworkAccessManager(this)), m_baseUrl(baseUrl)
{
}

void HermindClient::get(const QString &path, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QNetworkReply *reply = m_manager->get(req);

    connect(reply, &QNetworkReply::finished, [reply, callback]() {
        if (reply->error() != QNetworkReply::NoError) {
            callback(QJsonObject(), reply->errorString());
        } else {
            QByteArray data = reply->readAll();
            QJsonDocument doc = QJsonDocument::fromJson(data);
            callback(doc.object(), QString());
        }
        reply->deleteLater();
    });
}

void HermindClient::post(const QString &path, const QJsonObject &body, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QByteArray payload = QJsonDocument(body).toJson();
    QNetworkReply *reply = m_manager->post(req, payload);

    connect(reply, &QNetworkReply::finished, [reply, callback]() {
        if (reply->error() != QNetworkReply::NoError) {
            callback(QJsonObject(), reply->errorString());
        } else {
            QByteArray data = reply->readAll();
            QJsonDocument doc = QJsonDocument::fromJson(data);
            callback(doc.object(), QString());
        }
        reply->deleteLater();
    });
}

QNetworkReply* HermindClient::getStream(const QString &path)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    req.setRawHeader("Accept", "text/event-stream");
    return m_manager->get(req);
}

QString HermindClient::baseUrl() const
{
    return m_baseUrl;
}
