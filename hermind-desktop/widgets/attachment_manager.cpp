#include "attachment_manager.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>

AttachmentManager::AttachmentManager(QWidget *parent)
    : QWidget(parent)
{
    m_layout = new QHBoxLayout(this);
    m_layout->setContentsMargins(8, 4, 8, 0);
    m_layout->setSpacing(4);
    setVisible(false);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { rebuild(); });
}

QStringList AttachmentManager::filePaths() const { return m_files; }
int AttachmentManager::count() const { return m_files.size(); }

void AttachmentManager::addFiles(const QStringList &paths)
{
    for (const QString &p : paths) {
        if (!m_files.contains(p))
            m_files.append(p);
    }
    rebuild();
    emit attachmentsChanged(m_files);
}

void AttachmentManager::removeFile(const QString &path)
{
    m_files.removeAll(path);
    rebuild();
    emit attachmentsChanged(m_files);
}

void AttachmentManager::clear()
{
    m_files.clear();
    rebuild();
    emit attachmentsChanged(m_files);
}

void AttachmentManager::rebuild()
{
    // Clear existing chips
    QLayoutItem *child;
    while ((child = m_layout->takeAt(0)) != nullptr) {
        delete child->widget();
        delete child;
    }

    for (const QString &path : m_files) {
        QWidget *chip = new QWidget(this);
        QHBoxLayout *chipLayout = new QHBoxLayout(chip);
        chipLayout->setContentsMargins(6, 2, 6, 2);
        chipLayout->setSpacing(4);

        // Show just the filename
        QLabel *label = new QLabel(QFileInfo(path).fileName(), chip);
        label->setStyleSheet(QStringLiteral("color: palette(text); font-size: 12px;"));

        QPushButton *removeBtn = new QPushButton(QStringLiteral("×"), chip);
        removeBtn->setFixedSize(16, 16);
        removeBtn->setFlat(true);
        removeBtn->setCursor(Qt::PointingHandCursor);

        chipLayout->addWidget(label);
        chipLayout->addWidget(removeBtn);

        connect(removeBtn, &QPushButton::clicked, this, [this, path]() {
            removeFile(path);
        });

        const bool dark = ThemeManager::instance().isDarkMode();
        chip->setStyleSheet(QStringLiteral(
            "background-color: %1; border-radius: 8px;"
        ).arg(dark ? QStringLiteral("#3f3f46") : QStringLiteral("#e2e8f0")));

        m_layout->addWidget(chip);
    }
    m_layout->addStretch();

    setVisible(!m_files.isEmpty());
}
