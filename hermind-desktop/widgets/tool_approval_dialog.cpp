#include "tool_approval_dialog.h"
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>

ToolApprovalDialog::ToolApprovalDialog(const QString &requestId,
                                       const QString &skillName,
                                       const QString &description,
                                       QWidget *parent)
    : QDialog(parent)
    , m_requestId(requestId)
{
    setWindowTitle(QStringLiteral("工具审批"));
    setMinimumWidth(400);

    QVBoxLayout *layout = new QVBoxLayout(this);

    m_skillLabel = new QLabel(QStringLiteral("<b>技能:</b> %1").arg(skillName), this);
    m_skillLabel->setWordWrap(true);
    m_descLabel = new QLabel(QStringLiteral("<b>描述:</b> %1").arg(description), this);
    m_descLabel->setWordWrap(true);

    QHBoxLayout *btnLayout = new QHBoxLayout();
    m_rejectBtn = new QPushButton(QStringLiteral("拒绝"), this);
    m_approveBtn = new QPushButton(QStringLiteral("批准"), this);
    m_approveBtn->setDefault(true);

    btnLayout->addStretch();
    btnLayout->addWidget(m_rejectBtn);
    btnLayout->addWidget(m_approveBtn);

    layout->addWidget(m_skillLabel);
    layout->addWidget(m_descLabel);
    layout->addSpacing(12);
    layout->addLayout(btnLayout);

    connect(m_approveBtn, &QPushButton::clicked, this, [this]() {
        emit approved(m_requestId);
        accept();
    });
    connect(m_rejectBtn, &QPushButton::clicked, this, [this]() {
        emit rejected(m_requestId);
        reject();
    });
}
