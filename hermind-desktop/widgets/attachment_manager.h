#ifndef ATTACHMENT_MANAGER_H
#define ATTACHMENT_MANAGER_H

#include <QWidget>
#include <QStringList>
#include <QVector>

class QHBoxLayout;
class HermindApiClient;

class AttachmentManager : public QWidget
{
    Q_OBJECT
public:
    explicit AttachmentManager(QWidget *parent = nullptr);

    void setApiClient(HermindApiClient *client);
    void setWorkspaceSlug(const QString &slug);

    QStringList filePaths() const;
    // Base64 data URLs of attached images, ready to send with a chat message.
    // Non-image files are uploaded+embedded into the workspace instead and
    // are not part of the chat request (frontend DnDWrapper semantics).
    QStringList imageDataUrls() const;
    int count() const;
    // True while any non-image upload+embed is in flight; sending must wait.
    bool isProcessing() const;

signals:
    void attachmentsChanged(const QStringList &paths);
    void processingChanged(bool processing);

public slots:
    void addFiles(const QStringList &paths);
    void removeFile(const QString &path);
    void clear();

private:
    enum class Status { Ready, Uploading, Embedded, Failed };
    struct Entry {
        QString path;
        QString dataUrl; // images only
        QString docId;   // embedded documents only
        Status status = Status::Ready;
        QString error;
    };

    void rebuild();
    void startUpload(int index);
    void updateProcessingState();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    QVector<Entry> m_entries;
    bool m_processing = false;
    QHBoxLayout *m_layout = nullptr;
};

#endif // ATTACHMENT_MANAGER_H
