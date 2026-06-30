#include "api_response.h"

ApiError::ApiError(const QString &message,
                   int httpStatus,
                   QNetworkReply::NetworkError networkError)
    : m_message(message)
    , m_httpStatus(httpStatus)
    , m_networkError(networkError)
{
}

bool ApiError::isEmpty() const
{
    return m_message.isEmpty()
           && m_httpStatus == 0
           && m_networkError == QNetworkReply::NoError;
}

QString ApiError::message() const { return m_message; }
int ApiError::httpStatus() const { return m_httpStatus; }
QNetworkReply::NetworkError ApiError::networkError() const { return m_networkError; }

ApiResponse::ApiResponse(int statusCode, const QJsonDocument &body, const ApiError &error)
    : m_statusCode(statusCode)
    , m_body(body)
    , m_error(error)
{
}

bool ApiResponse::isSuccess() const
{
    return m_error.isEmpty() && m_statusCode >= 200 && m_statusCode < 300;
}

int ApiResponse::statusCode() const { return m_statusCode; }
QJsonDocument ApiResponse::body() const { return m_body; }
ApiError ApiResponse::error() const { return m_error; }
