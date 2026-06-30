#ifndef AUTH_STATE_H
#define AUTH_STATE_H

#include <QString>
#include "hermind_user.h"

enum class AuthState {
    Unauthenticated,   // 未登录或已登出
    Authenticating,    // 登录请求进行中
    Authenticated,     // 已登录（单用户/多用户均包含）
    Error              // 登录失败但尚未登出
};

struct AuthResult {
    bool success = false;
    QString token;
    HermindUser user;
    QString message;
};

#endif // AUTH_STATE_H
