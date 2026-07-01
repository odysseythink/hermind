#ifndef WORKSPACE_ITEM_WIDGET_H
#define WORKSPACE_ITEM_WIDGET_H

#include <QWidget>

class HermindWorkspace;
class QLabel;

class WorkspaceItemWidget : public QWidget
{
    Q_OBJECT

public:
    explicit WorkspaceItemWidget(QWidget *parent = nullptr);

    void setWorkspace(const HermindWorkspace &workspace);
    QString workspaceSlug() const;

    void setActive(bool active);
    bool isActive() const;

    // 仅用于测试触发点击
    void simulateClick();

signals:
    void workspaceClicked(const QString &slug);

protected:
    void mousePressEvent(QMouseEvent *event) override;
    void enterEvent(QEnterEvent *event) override;
    void leaveEvent(QEvent *event) override;

private:
    void applyStyle();

    HermindWorkspace *m_workspace = nullptr;
    QLabel *m_nameLabel = nullptr;
    bool m_active = false;
    bool m_hovered = false;
};

#endif // WORKSPACE_ITEM_WIDGET_H
