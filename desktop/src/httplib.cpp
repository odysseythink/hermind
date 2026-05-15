#include "httplib.h"
#include <QNetworkRequest>
#include <QUrl>
#include <QRandomGenerator>

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

void HermindClient::put(const QString &path, const QJsonObject &body, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QByteArray payload = QJsonDocument(body).toJson();
    QNetworkReply *reply = m_manager->put(req, payload);

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

void HermindClient::delete_(const QString &path, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QNetworkReply *reply = m_manager->deleteResource(req);

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

void HermindClient::upload(const QString &path, const QByteArray &data, const QString &fileName, const QString &mimeType, Callback callback)
{
    QString boundary = QString("----HermindBoundary%1").arg(QRandomGenerator::global()->generate(), 0, 16);

    QByteArray payload;
    payload.append(QString("--%1\r\n").arg(boundary).toUtf8());
    payload.append(QString("Content-Disposition: form-data; name=\"file\"; filename=\"%1\"\r\n").arg(fileName).toUtf8());
    payload.append(QString("Content-Type: %1\r\n").arg(mimeType).toUtf8());
    payload.append("\r\n");
    payload.append(data);
    payload.append(QString("\r\n--%1--\r\n").arg(boundary).toUtf8());

    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, QString("multipart/form-data; boundary=%1").arg(boundary));
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
