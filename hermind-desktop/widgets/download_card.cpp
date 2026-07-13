#include "download_card.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include <QVBoxLayout>
#include <QLabel>
#include <QDesktopServices>
#include <QUrl>

DownloadCard::DownloadCard(const QJsonObject &payload, QWidget *parent)
    : QWidget(parent)
{
    m_url = payload.value(QStringLiteral("url")).toString();
    QString name = payload.value(QStringLiteral("filename")).toString(
        payload.value(QStringLiteral("name")).toString());
    qint64 size = static_cast<qint64>(
        payload.value(QStringLiteral("size")).toDouble());

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(12, 10, 12, 10);

    m_nameLabel = new QLabel(name, this);
    m_nameLabel->setStyleSheet(QStringLiteral("font-weight: 600; font-size: 13px;"));

    QString sizeText;
    if (size > 1024 * 1024)
        sizeText = QStringLiteral("%1 MB").arg(size / (1024.0 * 1024.0), 0, 'f', 1);
    else if (size > 1024)
        sizeText = QStringLiteral("%1 KB").arg(size / 1024.0, 0, 'f', 0);
    else
        sizeText = QStringLiteral("%1 B").arg(size);

    m_sizeLabel = new QLabel(sizeText, this);

    layout->addWidget(m_nameLabel);
    layout->addWidget(m_sizeLabel);
    setCursor(Qt::PointingHandCursor);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void DownloadCard::mousePressEvent(QMouseEvent *event)
{
    QWidget::mousePressEvent(event);
    if (!m_url.isEmpty())
        QDesktopServices::openUrl(QUrl(m_url));
    emit clicked(m_url);
}

void DownloadCard::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "DownloadCard { background-color: %1; border-radius: 10px; border: 1px solid %2; }"
    ).arg(ThemeColors::cardBackground(dark).name(),
          ThemeColors::border(dark).name()));
}
