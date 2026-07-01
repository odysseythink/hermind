#ifndef NAVIGATION_MANAGER_H
#define NAVIGATION_MANAGER_H

#include <QObject>
#include <QVector>

#include "navigation_route.h"

class NavigationManager : public QObject
{
    Q_OBJECT

public:
    static NavigationManager &instance();

    NavigationRoute currentRoute() const;
    NavigationPage currentPage() const;
    bool canGoBack() const;
    const QVector<NavigationRoute> &history() const;

signals:
    void currentRouteChanged(const NavigationRoute &route);
    void canGoBackChanged(bool canGoBack);
    void historyChanged();

public slots:
    void navigateTo(const NavigationRoute &route);
    void goBack();
    void replaceWith(const NavigationRoute &route);
    void clearHistory();

private:
    explicit NavigationManager(QObject *parent = nullptr);
    ~NavigationManager() override = default;
    Q_DISABLE_COPY(NavigationManager)

    void setCurrentRoute(const NavigationRoute &route);
    void syncCanGoBack();
    void trimHistory();

    QVector<NavigationRoute> m_history;
    int m_currentIndex = -1;
    static constexpr int kMaxHistoryDepth = 50;
};

#endif // NAVIGATION_MANAGER_H
