#ifndef APPSTATE_H
#define APPSTATE_H

#include <QObject>
#include <QJsonObject>
#include <QJsonArray>
#include <QJsonValue>
#include <QNetworkReply>

class HermindCGOClient;
class SSEParser;

class AppState : public QObject
{
    Q_OBJECT
    Q_PROPERTY(QJsonObject config READ config NOTIFY configChanged)
    Q_PROPERTY(QJsonObject originalConfig READ originalConfig NOTIFY originalConfigChanged)
    Q_PROPERTY(QJsonArray configSections READ configSections NOTIFY configSectionsChanged)
    Q_PROPERTY(QString activeGroup READ activeGroup WRITE setActiveGroup NOTIFY activeGroupChanged)
    Q_PROPERTY(QString activeSubKey READ activeSubKey WRITE setActiveSubKey NOTIFY activeSubKeyChanged)
    Q_PROPERTY(int dirtyCount READ dirtyCount NOTIFY dirtyCountChanged)
    Q_PROPERTY(bool isStreaming READ isStreaming NOTIFY isStreamingChanged)
    Q_PROPERTY(QJsonArray messages READ messages NOTIFY messagesChanged)
    Q_PROPERTY(QJsonObject providerModels READ providerModels NOTIFY providerModelsChanged)
    Q_PROPERTY(QString status READ status NOTIFY statusChanged)
    Q_PROPERTY(QString flashMessage READ flashMessage NOTIFY flashMessageChanged)

public:
    explicit AppState(HermindCGOClient *client, QObject *parent = nullptr);

    QJsonObject config() const { return m_config; }
    QJsonObject originalConfig() const { return m_originalConfig; }
    QJsonArray configSections() const { return m_configSections; }
    QString activeGroup() const { return m_activeGroup; }
    void setActiveGroup(const QString &group);
    QString activeSubKey() const { return m_activeSubKey; }
    void setActiveSubKey(const QString &subKey);
    int dirtyCount() const { return m_dirtyCount; }
    bool isStreaming() const { return m_isStreaming; }
    QJsonArray messages() const { return m_messages; }
    QJsonObject providerModels() const { return m_providerModels; }
    QString status() const { return m_status; }
    QString flashMessage() const { return m_flashMessage; }

    Q_INVOKABLE void boot();
    Q_INVOKABLE void setConfigField(const QString &section, const QString &field, const QJsonValue &value);
    Q_INVOKABLE void setConfigScalar(const QString &section, const QJsonValue &value);
    Q_INVOKABLE void setKeyedField(const QString &section, const QString &subkey,
                                   const QString &instanceKey, const QString &field, const QJsonValue &value);
    Q_INVOKABLE void createKeyedInstance(const QString &section, const QString &subkey,
                                         const QString &instanceKey, const QJsonObject &initial);
    Q_INVOKABLE void deleteKeyedInstance(const QString &section, const QString &subkey, const QString &instanceKey);
    Q_INVOKABLE void setListField(const QString &section, const QString &subkey,
                                  int index, const QString &field, const QJsonValue &value);
    Q_INVOKABLE void createListInstance(const QString &section, const QString &subkey, const QJsonObject &initial);
    Q_INVOKABLE void deleteListInstance(const QString &section, const QString &subkey, int index);
    Q_INVOKABLE void moveListInstance(const QString &section, const QString &subkey, int index, const QString &direction);
    Q_INVOKABLE void saveConfig();
    Q_INVOKABLE void sendMessage(const QString &text);
    Q_INVOKABLE void cancelGeneration();
    Q_INVOKABLE void startNewSession();
    Q_INVOKABLE void fetchProviderModels(const QString &instanceKey);
    Q_INVOKABLE void testProvider(const QString &instanceKey);
    Q_INVOKABLE void testAuxiliary();
    Q_INVOKABLE void fetchAuxiliaryModels();
    Q_INVOKABLE void fetchFallbackModels(int index);
    Q_INVOKABLE void setLanguage(const QString &lang);
    void setClient(HermindCGOClient *client);

signals:
    void configChanged();
    void originalConfigChanged();
    void configSectionsChanged();
    void activeGroupChanged();
    void activeSubKeyChanged();
    void dirtyCountChanged();
    void isStreamingChanged();
    void messagesChanged();
    void providerModelsChanged();
    void statusChanged();
    void flashMessageChanged();
    void streamChunk(const QString &text);
    void streamDone();
    void streamError(const QString &error);
    void toast(const QString &message);
    void languageChanged(const QString &lang);

private:
    void startStream();
    void onStreamReadyRead();
    void onStreamFinished();
    void appendMessage(const QJsonObject &msg);
    void updateDirtyCount();

    HermindCGOClient *m_client;
    SSEParser *m_sseParser;
    QNetworkReply *m_streamReply;

    QJsonObject m_config;
    QJsonObject m_originalConfig;
    QJsonArray m_configSections;
    QString m_activeGroup;
    QString m_activeSubKey;
    int m_dirtyCount = 0;
    bool m_isStreaming = false;
    QJsonArray m_messages;
    QJsonObject m_providerModels;
    QString m_status = "booting";
    QString m_flashMessage;
};

#endif
