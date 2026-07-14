#ifndef AI_PROVIDER_SETTINGS_WIDGET_H
#define AI_PROVIDER_SETTINGS_WIDGET_H

#include <QHash>
#include <QWidget>

class HermindApiClient;
class QLabel;
class QStackedWidget;
class QButtonGroup;
class SidebarMenuButton;

// Frame for /settings/* AI provider pages (roadmap phase 3.0).
// Tabs: llm-preference, vector-database, embedding-preference,
// text-splitter-preference, audio-preference, transcription-preference,
// model-routers. Sub-phases 3.1-3.7 inject native or WebView pages
// via setTabWidget().
class AiProviderSettingsWidget : public QWidget
{
    Q_OBJECT

public:
    explicit AiProviderSettingsWidget(HermindApiClient *apiClient,
                                      QWidget *parent = nullptr);

    QString currentTabId() const;
    void setTabWidget(const QString &tabId, QWidget *widget);

public slots:
    void setActiveTab(const QString &tabId);
    // Single-user mode (empty role) and "admin" see all tabs;
    // any other role hides them (mirrors web roles: ["admin"]).
    void setUserRole(const QString &role);

signals:
    void returnClicked();
    void tabChanged(const QString &tabId);

private slots:
    void onTabButtonClicked();
    void applyStyle();

private:
    void buildUi();

    HermindApiClient *m_apiClient = nullptr;

    QStackedWidget *m_contentStack = nullptr;
    QLabel *m_headerTitleLabel = nullptr;
    QButtonGroup *m_tabGroup = nullptr;
    QHash<QString, SidebarMenuButton *> m_tabButtons;
    QString m_currentTabId;
};

#endif // AI_PROVIDER_SETTINGS_WIDGET_H
