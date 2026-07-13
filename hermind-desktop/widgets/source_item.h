#ifndef SOURCE_ITEM_H
#define SOURCE_ITEM_H

#include <QWidget>
#include <QJsonObject>

class QLabel;

class SourceItem : public QWidget
{
    Q_OBJECT
public:
    explicit SourceItem(const QJsonObject &source, QWidget *parent = nullptr);

signals:
    void clicked(const QJsonObject &source);

protected:
    void mousePressEvent(QMouseEvent *event) override;

private:
    void applyTheme();

    QJsonObject m_source;
    QLabel *m_titleLabel = nullptr;
    QLabel *m_descLabel = nullptr;
};

#endif // SOURCE_ITEM_H
