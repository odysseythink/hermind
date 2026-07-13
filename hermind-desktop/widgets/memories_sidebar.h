#ifndef MEMORIES_SIDEBAR_H
#define MEMORIES_SIDEBAR_H

#include <QWidget>
#include <QVector>
#include "hermind_memory.h"

class QVBoxLayout;
class QTabBar;
class QScrollArea;
class QPushButton;
class HermindApiClient;

class MemoriesSidebar : public QWidget
{
    Q_OBJECT
public:
    explicit MemoriesSidebar(HermindApiClient *apiClient, QWidget *parent = nullptr);

    void setWorkspace(const QString &slug, int workspaceId);
    void refresh();

    bool isOpen() const;

public slots:
    void open();
    void close();

signals:
    void closeRequested();

private:
    void applyTheme();
    void rebuildList();
    void fetchAndRefresh();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    int m_workspaceId = 0;
    bool m_open = false;

    QVector<HermindMemory> m_workspaceMemories;
    QVector<HermindMemory> m_globalMemories;

    QTabBar *m_tabBar = nullptr;
    QScrollArea *m_scroll = nullptr;
    QWidget *m_container = nullptr;
    QVBoxLayout *m_listLayout = nullptr;
    QPushButton *m_createBtn = nullptr;
    QPushButton *m_closeBtn = nullptr;
};

#endif // MEMORIES_SIDEBAR_H
