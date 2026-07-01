#ifndef ROUNDED_FRAME_H
#define ROUNDED_FRAME_H

#include <QFrame>

class RoundedFrame : public QFrame
{
    Q_OBJECT

public:
    explicit RoundedFrame(QWidget *parent = nullptr);

    void setRadius(int radius);

private:
    void applyStyle(bool dark);
    int m_radius = 16;
};

#endif // ROUNDED_FRAME_H
