#ifndef ATTACH_ITEM_H
#define ATTACH_ITEM_H

#include <QPushButton>

class AttachItem : public QPushButton
{
    Q_OBJECT
public:
    explicit AttachItem(QWidget *parent = nullptr);

signals:
    void filesSelected(const QStringList &paths);

private slots:
    void openFileDialog();
};

#endif // ATTACH_ITEM_H
