#ifndef PROMPTINPUT_H
#define PROMPTINPUT_H

#include <QWidget>

class QTextEdit;
class QPushButton;

class PromptInput : public QWidget
{
    Q_OBJECT
public:
    explicit PromptInput(QWidget *parent = nullptr);

signals:
    void sendClicked(QString text);
    void attachClicked();

private slots:
    void onSendClicked();

private:
    QTextEdit *m_textEdit;
    QPushButton *m_sendButton;
    QPushButton *m_attachButton;
};

#endif
