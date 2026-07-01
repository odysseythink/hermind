#ifndef SEARCH_INPUT_H
#define SEARCH_INPUT_H

#include <QLineEdit>

class SearchInput : public QLineEdit
{
    Q_OBJECT

public:
    explicit SearchInput(QWidget *parent = nullptr);

private:
    void applyStyle(bool dark);
};

#endif // SEARCH_INPUT_H
