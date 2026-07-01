#ifndef THREAD_CONTAINER_WIDGET_H
#define THREAD_CONTAINER_WIDGET_H

#include <QWidget>
#include <QVector>

#include "api_response.h"
#include "hermind_workspace_thread.h"

class HermindApiClient;
class QVBoxLayout;
class ThreadItemWidget;

class ThreadContainerWidget : public QWidget
{
    Q_OBJECT

public:
    explicit ThreadContainerWidget(QWidget *parent = nullptr);

    void setApiClient(HermindApiClient *apiClient);
    void setWorkspaceSlug(const QString &slug);
    void setSelectedThreadSlug(const QString &slug);
    void refresh();

signals:
    void threadClicked(const QString &workspaceSlug, const QString &threadSlug);
    void newThreadRequested(const QString &workspaceSlug);

protected:
    bool eventFilter(QObject *watched, QEvent *event) override;

private slots:
    void onThreadsLoaded(const QVector<HermindWorkspaceThread> &threads, const ApiError &error);
    void onThreadClicked(const QString &workspaceSlug, const QString &threadSlug);
    void onNewThreadClicked();
    void onRenameRequested(const QString &workspaceSlug, const QString &threadSlug);
    void onDeleteRequested(const QString &workspaceSlug, const QString &threadSlug);
    void onThreadRenamed(const HermindWorkspaceThread &thread, const QString &message, const ApiError &error);
    void onThreadDeleted(bool success, const ApiError &error);

private:
    void rebuildItems();
    ThreadItemWidget *findItem(const QString &threadSlug) const;

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    QString m_selectedThreadSlug;
    QVector<HermindWorkspaceThread> m_threads;
    QVBoxLayout *m_layout = nullptr;
    ThreadItemWidget *m_defaultItem = nullptr;
    QWidget *m_newThreadButton = nullptr;
    bool m_loading = false;
};

#endif // THREAD_CONTAINER_WIDGET_H
