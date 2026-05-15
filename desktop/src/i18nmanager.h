#ifndef I18NMANAGER_H
#define I18NMANAGER_H

#include <QObject>
#include <QTranslator>
#include <QLocale>

class I18nManager : public QObject
{
    Q_OBJECT
public:
    explicit I18nManager(QObject *parent = nullptr);
    void setLanguage(const QString &langCode);
    QString currentLanguage() const;
    static QString tr(const char *context, const char *text);

signals:
    void languageChanged(const QString &langCode);

private:
    QTranslator *m_translator;
    QString m_currentLang;
};

#endif
