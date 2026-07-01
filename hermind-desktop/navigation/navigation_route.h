#ifndef NAVIGATION_ROUTE_H
#define NAVIGATION_ROUTE_H

#include <QString>

enum class NavigationPage {
    DefaultChat,        // 首页 / 默认聊天
    WorkspaceChat,      // /workspace/:slug[/t/:threadSlug]
    WorkspaceSettings,  // /workspace/:slug/settings/:tab
    GeneralSettings,    // /settings/* （AI 提供商、全局设置等）
    AdminSettings,      // /settings/users、/settings/workspaces 等管理页
    Login,              // /login
    Onboarding,         // /onboarding[/:step]
    Invite,             // /accept-invite/:code
    NotFound            // *
};

struct NavigationRoute {
    NavigationPage page = NavigationPage::DefaultChat;
    QString workspaceSlug;   // WorkspaceChat / WorkspaceSettings 使用
    QString threadSlug;      // WorkspaceChat 使用
    QString settingsPath;    // GeneralSettings / AdminSettings / WorkspaceSettings 使用
    QString inviteCode;      // Invite 使用

    bool operator==(const NavigationRoute &other) const {
        return page == other.page &&
               workspaceSlug == other.workspaceSlug &&
               threadSlug == other.threadSlug &&
               settingsPath == other.settingsPath &&
               inviteCode == other.inviteCode;
    }

    bool operator!=(const NavigationRoute &other) const {
        return !(*this == other);
    }
};

#endif // NAVIGATION_ROUTE_H
