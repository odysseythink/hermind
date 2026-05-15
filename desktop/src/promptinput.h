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
    QString text() const;
    void insertText(const QString &text);
    void clear();
    void setEnabled(bool enabled);

signals:
    void sendClicked();
    void attachClicked();

private slots:
    void onSendClicked();

private:
    QTextEdit *m_textEdit;
    QPushButton *m_sendBtn;
    QPushButton *m_attachBtn;
};

#endif
