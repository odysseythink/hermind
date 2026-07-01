#ifndef STYLED_SEPARATOR_H
#define STYLED_SEPARATOR_H

#include <QFrame>

class StyledSeparator : public QFrame
{
    Q_OBJECT

public:
    explicit StyledSeparator(QWidget *parent = nullptr);

private:
    void applyStyle(bool dark);
};

#endif // STYLED_SEPARATOR_H
