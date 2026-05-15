#include "i18nmanager.h"
#include <QCoreApplication>

I18nManager::I18nManager(QObject *parent)
    : QObject(parent)
    , m_translator(new QTranslator(this))
    , m_currentLang(QStringLiteral("en"))
{
}

void I18nManager::setLanguage(const QString &langCode)
{
    if (m_currentLang == langCode)
        return;

    QCoreApplication::removeTranslator(m_translator);

    if (langCode != QStringLiteral("en")) {
        QString qmPath = QStringLiteral(":/i18n/%1.qm").arg(langCode);
        if (m_translator->load(qmPath)) {
            QCoreApplication::installTranslator(m_translator);
        }
    }

    m_currentLang = langCode;
    emit languageChanged(langCode);
}

QString I18nManager::currentLanguage() const
{
    return m_currentLang;
}

QString I18nManager::tr(const char *context, const char *text)
{
    return QCoreApplication::translate(context, text);
}
