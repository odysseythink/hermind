#ifndef TOOL_APPROVAL_DIALOG_H
#define TOOL_APPROVAL_DIALOG_H

#include <QDialog>

class QLabel;
class QPushButton;

class ToolApprovalDialog : public QDialog
{
    Q_OBJECT
public:
    explicit ToolApprovalDialog(const QString &requestId,
                                const QString &skillName,
                                const QString &description,
                                QWidget *parent = nullptr);

signals:
    void approved(const QString &requestId);
    void rejected(const QString &requestId);

private:
    QString m_requestId;
    QLabel *m_skillLabel = nullptr;
    QLabel *m_descLabel = nullptr;
    QPushButton *m_approveBtn = nullptr;
    QPushButton *m_rejectBtn = nullptr;
};

#endif // TOOL_APPROVAL_DIALOG_H
