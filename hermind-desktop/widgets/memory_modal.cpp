#include "memory_modal.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QTextEdit>
#include <QPushButton>

MemoryModal::MemoryModal(Mode mode, const QString &initialContent, QWidget *parent)
    : QDialog(parent)
    , m_mode(mode)
{
    setWindowTitle(mode == Create ? QStringLiteral("创建记忆") : QStringLiteral("编辑记忆"));
    setMinimumSize(400, 250);

    QVBoxLayout *layout = new QVBoxLayout(this);

    m_edit = new QTextEdit(this);
    m_edit->setPlainText(initialContent);
    m_edit->setPlaceholderText(QStringLiteral("输入记忆内容..."));
    layout->addWidget(m_edit, 1);

    QHBoxLayout *btnLayout = new QHBoxLayout();
    QPushButton *cancelBtn = new QPushButton(QStringLiteral("取消"), this);
    QPushButton *submitBtn = new QPushButton(
        mode == Create ? QStringLiteral("创建") : QStringLiteral("保存"), this);
    submitBtn->setDefault(true);

    btnLayout->addStretch();
    btnLayout->addWidget(cancelBtn);
    btnLayout->addWidget(submitBtn);
    layout->addLayout(btnLayout);

    connect(cancelBtn, &QPushButton::clicked, this, &QDialog::reject);
    connect(submitBtn, &QPushButton::clicked, this, [this]() {
        QString content = m_edit->toPlainText().trimmed();
        if (!content.isEmpty()) {
            emit submitted(content);
            accept();
        }
    });
}
