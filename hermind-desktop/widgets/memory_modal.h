#ifndef MEMORY_MODAL_H
#define MEMORY_MODAL_H

#include <QDialog>

class QTextEdit;

class MemoryModal : public QDialog
{
    Q_OBJECT
public:
    enum Mode { Create, Edit };

    explicit MemoryModal(Mode mode, const QString &initialContent = QString(),
                         QWidget *parent = nullptr);

signals:
    void submitted(const QString &content);

private:
    Mode m_mode;
    QTextEdit *m_edit = nullptr;
};

#endif // MEMORY_MODAL_H
