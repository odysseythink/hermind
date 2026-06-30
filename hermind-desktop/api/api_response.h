#ifndef API_RESPONSE_H
#define API_RESPONSE_H

#include <QString>
#include <QJsonDocument>
#include <QNetworkReply>

class ApiError
{
public:
    ApiError() = default;
    explicit ApiError(const QString &message,
                      int httpStatus = 0,
                      QNetworkReply::NetworkError networkError = QNetworkReply::NoError);

    bool isEmpty() const;

    QString message() const;
    int httpStatus() const;
    QNetworkReply::NetworkError networkError() const;

private:
    QString m_message;
    int m_httpStatus = 0;
    QNetworkReply::NetworkError m_networkError = QNetworkReply::NoError;
};

class ApiResponse
{
public:
    ApiResponse() = default;
    ApiResponse(int statusCode, const QJsonDocument &body, const ApiError &error = ApiError());

    bool isSuccess() const;

    int statusCode() const;
    QJsonDocument body() const;
    ApiError error() const;

private:
    int m_statusCode = 0;
    QJsonDocument m_body;
    ApiError m_error;
};

#endif // API_RESPONSE_H
