#include <QtTest>
#include <QSignalSpy>
#include <QApplication>
#include <QStyleFactory>
#include "theme_manager.h"
#include "settings_store.h"

class TestThemeManager : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void init();
    void cleanup();

    void singletonReturnsSameInstance();
    void defaultThemeIsSystem();
    void defaultLanguageIsEn();
    void setThemeChangesTheme();
    void setThemeEmitsSignal();
    void setThemePersistsToSettings();
    void effectiveThemeResolvesSystem();
    void effectiveThemePassthroughLight();
    void effectiveThemePassthroughDark();
    void isDarkModeCorrect();
    void setLanguageChangesLanguage();
    void setLanguageEmitsSignal();
    void setLanguagePersistsToSettings();
    void setThemeNoopSameValue();
    void setLanguageNoopSameValue();
    void detectSystemThemeReturnsLightOrDark();
};

void TestThemeManager::initTestCase()
{
    // Signal spies need the object to exist
}

void TestThemeManager::cleanupTestCase()
{
}

void TestThemeManager::init()
{
    SettingsStore::instance().clear();
    ThemeManager::instance().initialize(&SettingsStore::instance(), nullptr);
}

void TestThemeManager::cleanup()
{
    SettingsStore::instance().clear();
    ThemeManager::instance().initialize(&SettingsStore::instance(), nullptr);
}

// --- Singleton ---
void TestThemeManager::singletonReturnsSameInstance()
{
    ThemeManager &a = ThemeManager::instance();
    ThemeManager &b = ThemeManager::instance();
    QCOMPARE(&a, &b);
}

// --- Defaults ---
void TestThemeManager::defaultThemeIsSystem()
{
    QCOMPARE(ThemeManager::instance().theme(), QStringLiteral("system"));
}

void TestThemeManager::defaultLanguageIsEn()
{
    QCOMPARE(ThemeManager::instance().language(), QStringLiteral("en"));
}

// --- setTheme ---
void TestThemeManager::setThemeChangesTheme()
{
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QCOMPARE(ThemeManager::instance().theme(), QStringLiteral("dark"));

    ThemeManager::instance().setTheme(QStringLiteral("light"));
    QCOMPARE(ThemeManager::instance().theme(), QStringLiteral("light"));
}

void TestThemeManager::setThemeEmitsSignal()
{
    QSignalSpy spy(&ThemeManager::instance(), &ThemeManager::themeChanged);
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).toString(), QStringLiteral("dark"));

    ThemeManager::instance().setTheme(QStringLiteral("light"));
    QCOMPARE(spy.count(), 2);
    QCOMPARE(spy.at(1).at(0).toString(), QStringLiteral("light"));
}

void TestThemeManager::setThemePersistsToSettings()
{
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QCOMPARE(SettingsStore::instance().theme(), QStringLiteral("dark"));

    ThemeManager::instance().setTheme(QStringLiteral("light"));
    QCOMPARE(SettingsStore::instance().theme(), QStringLiteral("light"));
}

// --- effectiveTheme ---
void TestThemeManager::effectiveThemeResolvesSystem()
{
    // With theme="system", effectiveTheme returns detectSystemTheme()
    ThemeManager::instance().setTheme(QStringLiteral("system"));
    const QString effective = ThemeManager::instance().effectiveTheme();
    QVERIFY(effective == QLatin1String("light") || effective == QLatin1String("dark"));
}

void TestThemeManager::effectiveThemePassthroughLight()
{
    ThemeManager::instance().setTheme(QStringLiteral("light"));
    QCOMPARE(ThemeManager::instance().effectiveTheme(), QStringLiteral("light"));
}

void TestThemeManager::effectiveThemePassthroughDark()
{
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QCOMPARE(ThemeManager::instance().effectiveTheme(), QStringLiteral("dark"));
}

// --- isDarkMode ---
void TestThemeManager::isDarkModeCorrect()
{
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QVERIFY(ThemeManager::instance().isDarkMode());

    ThemeManager::instance().setTheme(QStringLiteral("light"));
    QVERIFY(!ThemeManager::instance().isDarkMode());
}

// --- setLanguage ---
void TestThemeManager::setLanguageChangesLanguage()
{
    ThemeManager::instance().setLanguage(QStringLiteral("zh"));
    QCOMPARE(ThemeManager::instance().language(), QStringLiteral("zh"));

    ThemeManager::instance().setLanguage(QStringLiteral("en"));
    QCOMPARE(ThemeManager::instance().language(), QStringLiteral("en"));
}

void TestThemeManager::setLanguageEmitsSignal()
{
    QSignalSpy spy(&ThemeManager::instance(), &ThemeManager::languageChanged);
    ThemeManager::instance().setLanguage(QStringLiteral("zh"));
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).toString(), QStringLiteral("zh"));

    ThemeManager::instance().setLanguage(QStringLiteral("en"));
    QCOMPARE(spy.count(), 2);
    QCOMPARE(spy.at(1).at(0).toString(), QStringLiteral("en"));
}

void TestThemeManager::setLanguagePersistsToSettings()
{
    ThemeManager::instance().setLanguage(QStringLiteral("zh"));
    QCOMPARE(SettingsStore::instance().language(), QStringLiteral("zh"));

    ThemeManager::instance().setLanguage(QStringLiteral("en"));
    QCOMPARE(SettingsStore::instance().language(), QStringLiteral("en"));
}

// --- No-op when same value ---
void TestThemeManager::setThemeNoopSameValue()
{
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QSignalSpy spy(&ThemeManager::instance(), &ThemeManager::themeChanged);
    ThemeManager::instance().setTheme(QStringLiteral("dark"));
    QCOMPARE(spy.count(), 0);
}

void TestThemeManager::setLanguageNoopSameValue()
{
    ThemeManager::instance().setLanguage(QStringLiteral("zh"));
    QSignalSpy spy(&ThemeManager::instance(), &ThemeManager::languageChanged);
    ThemeManager::instance().setLanguage(QStringLiteral("zh"));
    QCOMPARE(spy.count(), 0);
}

// --- detectSystemTheme ---
void TestThemeManager::detectSystemThemeReturnsLightOrDark()
{
    const QString result = ThemeManager::detectSystemTheme();
    QVERIFY(result == QLatin1String("light") || result == QLatin1String("dark"));
}

QTEST_MAIN(TestThemeManager)
#include "tst_theme_manager.moc"
