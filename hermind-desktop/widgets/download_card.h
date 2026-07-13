#ifndef DOWNLOAD_CARD_H
#define DOWNLOAD_CARD_H

#include <QWidget>
#include <QJsonObject>

class QLabel;

class DownloadCard : public QWidget
{
    Q_OBJECT
public:
    explicit DownloadCard(const QJsonObject &payload, QWidget *parent = nullptr);

signals:
    void clicked(const QString &url);

protected:
    void mousePressEvent(QMouseEvent *event) override;

private:
    void applyTheme();

    QString m_url;
    QLabel *m_nameLabel = nullptr;
    QLabel *m_sizeLabel = nullptr;
};

#endif // DOWNLOAD_CARD_H
