#include "toolcallwidget.h"

#include <QLabel>
#include <QHBoxLayout>

ToolCallWidget::ToolCallWidget(const QString &name, const QString &status, QWidget *parent)
    : QWidget(parent),
      m_name(name),
      m_status(status)
{
    setupUI();
    updateAppearance();
}

void ToolCallWidget::setupUI()
{
    m_iconLabel = new QLabel(this);
    m_nameLabel = new QLabel(m_name, this);
    m_statusLabel = new QLabel(this);

    m_nameLabel->setStyleSheet(QStringLiteral("font-size: 12px; color: #e8e6e3;"));
    m_statusLabel->setStyleSheet(QStringLiteral("font-size: 11px;"));

    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(8, 4, 8, 4);
    layout->setSpacing(8);
    layout->addWidget(m_iconLabel);
    layout->addWidget(m_nameLabel, 1);
    layout->addWidget(m_statusLabel);

    setStyleSheet(QStringLiteral(
        "ToolCallWidget {"
        "  background: #1a1c20;"
        "  border: 1px solid #2a2e36;"
        "  border-radius: 4px;"
        "}"
    ));
}

void ToolCallWidget::updateAppearance()
{
    if (m_status == QStringLiteral("running") || m_status == QStringLiteral("started")) {
        m_iconLabel->setText(QStringLiteral("●"));
        m_iconLabel->setStyleSheet(QStringLiteral("color: #FFB800; font-size: 10px;"));
        m_statusLabel->setText(QStringLiteral("Running…"));
        m_statusLabel->setStyleSheet(QStringLiteral("color: #FFB800; font-size: 11px;"));
    } else if (m_status == QStringLiteral("completed") || m_status == QStringLiteral("success")) {
        m_iconLabel->setText(QStringLiteral("✓"));
        m_iconLabel->setStyleSheet(QStringLiteral("color: #6a9955; font-size: 12px;"));
        m_statusLabel->setText(QStringLiteral("Done"));
        m_statusLabel->setStyleSheet(QStringLiteral("color: #6a9955; font-size: 11px;"));
    } else {
        m_iconLabel->setText(QStringLiteral("✗"));
        m_iconLabel->setStyleSheet(QStringLiteral("color: #ce9178; font-size: 12px;"));
        m_statusLabel->setText(QStringLiteral("Failed"));
        m_statusLabel->setStyleSheet(QStringLiteral("color: #ce9178; font-size: 11px;"));
    }
}

void ToolCallWidget::setStatus(const QString &status)
{
    if (m_status == status)
        return;
    m_status = status;
    updateAppearance();
}

QString ToolCallWidget::name() const
{
    return m_name;
}
