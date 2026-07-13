#ifndef MEMORY_CARD_H
#define MEMORY_CARD_H

#include <QWidget>
#include "hermind_memory.h"

class QLabel;

class MemoryCard : public QWidget
{
    Q_OBJECT
public:
    explicit MemoryCard(const HermindMemory &memory, QWidget *parent = nullptr);

signals:
    void editRequested(const HermindMemory &memory);
    void deleteRequested(int memoryId);
    void promoteRequested(int memoryId);
    void demoteRequested(int memoryId);

private:
    void applyTheme();

    HermindMemory m_memory;
    QLabel *m_contentLabel = nullptr;
};

#endif // MEMORY_CARD_H
