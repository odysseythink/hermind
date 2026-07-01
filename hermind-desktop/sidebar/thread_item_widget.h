#ifndef THREAD_ITEM_WIDGET_H
#define THREAD_ITEM_WIDGET_H

#include <QWidget>

class HermindWorkspaceThread;
class QLabel;

class ThreadItemWidget : public QWidget
{
    Q_OBJECT

public:
    explicit ThreadItemWidget(QWidget *parent = nullptr);

    void setThread(const HermindWorkspaceThread &thread);
    void setWorkspaceSlug(const QString &slug);
    void setDefaultThread(bool isDefault);
    void setSelected(bool selected);
    bool isSelected() const;

    QString workspaceSlug() const;
    QString threadSlug() const;
    bool isDefaultThread() const;

    // 仅用于测试触发点击
    void simulateClick();
    void simulateRenameRequest();
    void simulateDeleteRequest();

signals:
    void threadClicked(const QString &workspaceSlug, const QString &threadSlug);
    void renameRequested(const QString &workspaceSlug, const QString &threadSlug);
    void deleteRequested(const QString &workspaceSlug, const QString &threadSlug);

protected:
    void mousePressEvent(QMouseEvent *event) override;
    void enterEvent(QEnterEvent *event) override;
    void leaveEvent(QEvent *event) override;
    void contextMenuEvent(QContextMenuEvent *event) override;

private:
    void applyStyle();
    void updateNameLabel();

    QString m_workspaceSlug;
    QString m_threadSlug;
    QString m_name;
    bool m_isDefault = false;
    bool m_selected = false;
    bool m_hovered = false;
    QLabel *m_nameLabel = nullptr;
};

#endif // THREAD_ITEM_WIDGET_H
