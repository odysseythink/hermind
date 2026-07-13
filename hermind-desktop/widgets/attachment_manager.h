#ifndef ATTACHMENT_MANAGER_H
#define ATTACHMENT_MANAGER_H

#include <QWidget>
#include <QStringList>

class QHBoxLayout;

class AttachmentManager : public QWidget
{
    Q_OBJECT
public:
    explicit AttachmentManager(QWidget *parent = nullptr);

    QStringList filePaths() const;
    int count() const;

signals:
    void attachmentsChanged(const QStringList &paths);

public slots:
    void addFiles(const QStringList &paths);
    void removeFile(const QString &path);
    void clear();

private:
    void rebuild();

    QStringList m_files;
    QHBoxLayout *m_layout = nullptr;
};

#endif // ATTACHMENT_MANAGER_H
