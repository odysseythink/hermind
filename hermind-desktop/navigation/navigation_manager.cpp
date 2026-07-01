#include "navigation_manager.h"

namespace {

NavigationRoute defaultRoute()
{
    NavigationRoute route;
    route.page = NavigationPage::DefaultChat;
    return route;
}

} // namespace

NavigationManager &NavigationManager::instance()
{
    static NavigationManager mgr;
    return mgr;
}

NavigationManager::NavigationManager(QObject *parent)
    : QObject(parent)
{
    setCurrentRoute(defaultRoute());
}

NavigationRoute NavigationManager::currentRoute() const
{
    if (m_currentIndex < 0 || m_currentIndex >= m_history.size())
        return defaultRoute();
    return m_history.at(m_currentIndex);
}

NavigationPage NavigationManager::currentPage() const
{
    return currentRoute().page;
}

bool NavigationManager::canGoBack() const
{
    return m_currentIndex > 0;
}

const QVector<NavigationRoute> &NavigationManager::history() const
{
    return m_history;
}

void NavigationManager::navigateTo(const NavigationRoute &route)
{
    const bool oldCanGoBack = canGoBack();

    // Truncate any forward history.
    if (m_currentIndex >= 0 && m_currentIndex < m_history.size() - 1) {
        m_history.resize(m_currentIndex + 1);
    }

    m_history.append(route);
    m_currentIndex = m_history.size() - 1;
    trimHistory();

    emit currentRouteChanged(route);
    emit historyChanged();
    if (oldCanGoBack != canGoBack())
        emit canGoBackChanged(canGoBack());
}

void NavigationManager::goBack()
{
    if (!canGoBack())
        return;

    const bool oldCanGoBack = canGoBack();
    --m_currentIndex;
    const NavigationRoute route = currentRoute();

    emit currentRouteChanged(route);
    emit historyChanged();
    if (oldCanGoBack != canGoBack())
        emit canGoBackChanged(canGoBack());
}

void NavigationManager::replaceWith(const NavigationRoute &route)
{
    if (m_currentIndex < 0 || m_currentIndex >= m_history.size()) {
        navigateTo(route);
        return;
    }

    m_history.replace(m_currentIndex, route);
    emit currentRouteChanged(route);
    emit historyChanged();
}

void NavigationManager::clearHistory()
{
    const bool oldCanGoBack = canGoBack();
    m_history.clear();
    m_currentIndex = 0;
    m_history.append(defaultRoute());
    emit historyChanged();
    if (oldCanGoBack != canGoBack())
        emit canGoBackChanged(canGoBack());
}

void NavigationManager::setCurrentRoute(const NavigationRoute &route)
{
    if (m_currentIndex >= 0 && m_currentIndex < m_history.size() &&
        m_history.at(m_currentIndex) == route) {
        return;
    }

    if (m_currentIndex < 0 || m_currentIndex >= m_history.size()) {
        m_history.append(route);
        m_currentIndex = 0;
    } else {
        m_history.replace(m_currentIndex, route);
    }

    emit currentRouteChanged(route);
    emit historyChanged();
    syncCanGoBack();
}

void NavigationManager::syncCanGoBack()
{
    // No cached previous value; used only during initialization.
    // Public consumers should read canGoBack() after receiving signals.
}

void NavigationManager::trimHistory()
{
    if (m_history.size() <= kMaxHistoryDepth + 1)
        return;

    const int excess = m_history.size() - (kMaxHistoryDepth + 1);
    m_history.remove(0, excess);
    m_currentIndex -= excess;
    if (m_currentIndex < 0)
        m_currentIndex = 0;
}
