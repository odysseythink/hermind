#include "attachment_manager.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QFile>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QMimeDatabase>
#include <QPointer>
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

void AttachmentManager::setApiClient(HermindApiClient *client) { m_apiClient = client; }
void AttachmentManager::setWorkspaceSlug(const QString &slug) { m_workspaceSlug = slug; }

QStringList AttachmentManager::filePaths() const
{
    QStringList paths;
    for (const Entry &e : m_entries)
        paths.append(e.path);
    return paths;
}

QStringList AttachmentManager::imageDataUrls() const
{
    QStringList urls;
    for (const Entry &e : m_entries) {
        if (!e.dataUrl.isEmpty())
            urls.append(e.dataUrl);
    }
    return urls;
}

int AttachmentManager::count() const { return m_entries.size(); }

bool AttachmentManager::isProcessing() const
{
    for (const Entry &e : m_entries) {
        if (e.status == Status::Uploading)
            return true;
    }
    return false;
}

void AttachmentManager::addFiles(const QStringList &paths)
{
    const QMimeDatabase mimeDb;
    bool added = false;
    for (const QString &p : paths) {
        bool dup = false;
        for (const Entry &e : m_entries) {
            if (e.path == p) {
                dup = true;
                break;
            }
        }
        if (dup)
            continue;

        Entry entry;
        entry.path = p;
        const QString mime = mimeDb.mimeTypeForFile(p).name();
        if (mime.startsWith(QLatin1String("image/"))) {
            // Images go inline as base64 data URLs with the chat request.
            QFile f(p);
            if (f.open(QIODevice::ReadOnly)) {
                entry.dataUrl = QStringLiteral("data:%1;base64,%2")
                                    .arg(mime, QString::fromLatin1(f.readAll().toBase64()));
                entry.status = Status::Ready;
            } else {
                entry.status = Status::Failed;
                entry.error = tr("Cannot read file");
            }
            m_entries.append(entry);
        } else {
            // Non-image files are uploaded + embedded into the workspace.
            if (m_apiClient && !m_workspaceSlug.isEmpty()) {
                entry.status = Status::Uploading;
                m_entries.append(entry);
                startUpload(m_entries.size() - 1);
            } else {
                entry.status = Status::Failed;
                entry.error = tr("No workspace selected");
                m_entries.append(entry);
            }
        }
        added = true;
    }
    if (!added)
        return;
    rebuild();
    updateProcessingState();
    emit attachmentsChanged(filePaths());
}

void AttachmentManager::removeFile(const QString &path)
{
    for (int i = 0; i < m_entries.size(); ++i) {
        if (m_entries[i].path != path)
            continue;
        const Entry entry = m_entries.takeAt(i);
        if (!entry.docId.isEmpty() && m_apiClient && !m_workspaceSlug.isEmpty()) {
            const QString slug = m_workspaceSlug;
            m_apiClient->removeAndUnembed(slug, entry.docId, [](bool, const ApiError &) {});
        }
        rebuild();
        updateProcessingState();
        emit attachmentsChanged(filePaths());
        return;
    }
}

void AttachmentManager::clear()
{
    // Embedded documents stay in the workspace (frontend resetAttachments
    // semantics); only explicit chip removal un-embeds them.
    m_entries.clear();
    rebuild();
    updateProcessingState();
    emit attachmentsChanged(filePaths());
}

void AttachmentManager::startUpload(int index)
{
    const QString path = m_entries[index].path;
    const QString slug = m_workspaceSlug;
    QPointer<AttachmentManager> guard(this);
    m_apiClient->uploadAndEmbedFile(
        slug, path, [guard, path](const QJsonObject &document, const ApiError &error) {
            if (!guard)
                return;
            AttachmentManager *self = guard.data();
            for (int i = 0; i < self->m_entries.size(); ++i) {
                if (self->m_entries[i].path != path)
                    continue;
                const QString docId = document.value(QStringLiteral("docId")).toString();
                if (error.isEmpty() && !docId.isEmpty()) {
                    self->m_entries[i].status = Status::Embedded;
                    self->m_entries[i].docId = docId;
                } else {
                    self->m_entries[i].status = Status::Failed;
                    self->m_entries[i].error = error.isEmpty() ? tr("Upload failed")
                                                               : error.message();
                }
                break;
            }
            self->rebuild();
            self->updateProcessingState();
        });
}

void AttachmentManager::updateProcessingState()
{
    const bool now = isProcessing();
    if (now == m_processing)
        return;
    m_processing = now;
    emit processingChanged(now);
}

void AttachmentManager::rebuild()
{
    // Clear existing chips
    QLayoutItem *child;
    while ((child = m_layout->takeAt(0)) != nullptr) {
        delete child->widget();
        delete child;
    }

    const bool dark = ThemeManager::instance().isDarkMode();
    for (const Entry &entry : m_entries) {
        QWidget *chip = new QWidget(this);
        QHBoxLayout *chipLayout = new QHBoxLayout(chip);
        chipLayout->setContentsMargins(6, 2, 6, 2);
        chipLayout->setSpacing(4);

        QString statusText;
        switch (entry.status) {
        case Status::Uploading:
            statusText = tr("Uploading...");
            break;
        case Status::Embedded:
            statusText = tr("Embedded");
            break;
        case Status::Failed:
            statusText = entry.error.isEmpty() ? tr("Failed") : entry.error;
            break;
        case Status::Ready:
            break;
        }

        QLabel *label = new QLabel(chip);
        label->setText(statusText.isEmpty()
                           ? QFileInfo(entry.path).fileName()
                           : QStringLiteral("%1 (%2)").arg(QFileInfo(entry.path).fileName(), statusText));
        label->setStyleSheet(QStringLiteral("color: palette(text); font-size: 12px;"));

        QPushButton *removeBtn = new QPushButton(QStringLiteral("×"), chip);
        removeBtn->setFixedSize(16, 16);
        removeBtn->setFlat(true);
        removeBtn->setCursor(Qt::PointingHandCursor);

        chipLayout->addWidget(label);
        chipLayout->addWidget(removeBtn);

        const QString path = entry.path;
        connect(removeBtn, &QPushButton::clicked, this, [this, path]() {
            removeFile(path);
        });

        chip->setStyleSheet(QStringLiteral(
            "background-color: %1; border-radius: 8px;"
        ).arg(dark ? QStringLiteral("#3f3f46") : QStringLiteral("#e2e8f0")));

        m_layout->addWidget(chip);
    }
    m_layout->addStretch();

    setVisible(!m_entries.isEmpty());
}
