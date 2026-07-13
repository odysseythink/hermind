#include "attach_item.h"
#include <QFileDialog>

AttachItem::AttachItem(QWidget *parent)
    : QPushButton(parent)
{
    setText(QStringLiteral("+"));
    setFixedSize(32, 32);
    setCursor(Qt::PointingHandCursor);
    setToolTip(QStringLiteral("添加附件"));
    setFlat(true);
    connect(this, &QPushButton::clicked, this, &AttachItem::openFileDialog);
}

void AttachItem::openFileDialog()
{
    QStringList files = QFileDialog::getOpenFileNames(
        this, QStringLiteral("选择文件"), QString(),
        QStringLiteral("所有文件 (*.*)"));
    if (!files.isEmpty())
        emit filesSelected(files);
}
