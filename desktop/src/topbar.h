#ifndef TOPBAR_H
#define TOPBAR_H

#include <QWidget>

class QLabel;
class QPushButton;
class QComboBox;

class TopBar : public QWidget
{
    Q_OBJECT
public:
    explicit TopBar(QWidget *parent = nullptr);

    void setStatus(const QString &status);
    void setStatusDotColor(const QString &color);
    void setLanguage(const QString &langCode);
    void setThemeDark(bool dark);

signals:
    void modeChanged(const QString &mode);
    void saveRequested();
    void languageChanged(const QString &langCode);
    void themeToggled();

private:
    void setupUI();

    QLabel *m_brandLabel;
    QPushButton *m_chatModeBtn;
    QPushButton *m_settingsModeBtn;
    QLabel *m_statusLabel;
    QWidget *m_statusDot;
    QPushButton *m_saveBtn;
    QComboBox *m_langCombo;
    QPushButton *m_themeBtn;
};

#endif
