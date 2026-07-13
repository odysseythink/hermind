#ifndef WORKSPACE_ITEM_WIDGET_H
#define WORKSPACE_ITEM_WIDGET_H

#include <QWidget>

#include "hermind_workspace.h"

class HermindApiClient;
class QLabel;
class ThreadContainerWidget;

class WorkspaceItemWidget : public QWidget
{
    Q_OBJECT

public:
    explicit WorkspaceItemWidget(QWidget *parent = nullptr);

    void setWorkspace(const HermindWorkspace &workspace);
    QString workspaceSlug() const;

    void setActive(bool active);
    bool isActive() const;

    void setExpanded(bool expanded);
    bool isExpanded() const;

    void setApiClient(HermindApiClient *apiClient);
    void setSelectedThreadSlug(const QString &slug);
    void refreshThreads();

    // 仅用于测试触发点击
    void simulateClick();

signals:
    void workspaceClicked(const QString &slug);
    void threadClicked(const QString &workspaceSlug, const QString &threadSlug);
    void workspaceSettingsRequested(const QString &slug);
    void uploadDocumentsRequested(const QString &slug);

protected:
    void enterEvent(QEnterEvent *event) override;
    void leaveEvent(QEvent *event) override;
    bool eventFilter(QObject *watched, QEvent *event) override;

private slots:
    void applyStyle();
    void onSettingsClicked();
    void onUploadClicked();

private:
    void updateExpandArrow();

    HermindWorkspace m_workspace;
    QLabel *m_nameLabel = nullptr;
    QLabel *m_expandLabel = nullptr;
    QWidget *m_header = nullptr;
    QWidget *m_content = nullptr;
    ThreadContainerWidget *m_threadContainer = nullptr;
    bool m_active = false;
    bool m_hovered = false;
    bool m_expanded = false;
};

#endif // WORKSPACE_ITEM_WIDGET_H
