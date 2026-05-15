#include "statusfooter.h"

#include <QLabel>
#include <QHBoxLayout>

StatusFooter::StatusFooter(QWidget *parent)
    : QWidget(parent),
      m_label(new QLabel(this))
{
    setFixedHeight(32);
    setupUI();
    setVersion("v0.3.0");
    setModel("Qt6 Desktop");
    setStatus("Ready");
}

void StatusFooter::setupUI()
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 0, 16, 0);
    layout->setSpacing(0);

    m_label->setStyleSheet(
        "font-family: monospace; font-size: 11px; color: #8a8680;"
    );
    layout->addWidget(m_label);
    layout->addStretch(1);
}

void StatusFooter::setVersion(const QString &version)
{
    m_label->setText(QString("◈ hermind %1 · %2 · %3")
                     .arg(version, "Qt6 Desktop", m_label->text().split("·").last().trimmed()));
}

void StatusFooter::setModel(const QString &model)
{
    Q_UNUSED(model)
}

void StatusFooter::setStatus(const QString &status)
{
    QStringList parts = m_label->text().split("·");
    if (parts.size() >= 3) {
        parts[2] = " " + status;
        m_label->setText(parts.join("·"));
    } else {
        m_label->setText(QString("◈ hermind v0.3.0 · Qt6 Desktop · %1").arg(status));
    }
}
