#ifndef QUICK_ACTIONS_H
#define QUICK_ACTIONS_H

#include <QWidget>

class QPushButton;

class QuickActions : public QWidget
{
    Q_OBJECT
public:
    explicit QuickActions(QWidget *parent = nullptr);

signals:
    void createAgentClicked();
    void editWorkspaceClicked();
    void uploadDocumentClicked();

private:
    void applyTheme();

    QPushButton *m_createAgentBtn = nullptr;
    QPushButton *m_editWorkspaceBtn = nullptr;
    QPushButton *m_uploadBtn = nullptr;
};

#endif // QUICK_ACTIONS_H
