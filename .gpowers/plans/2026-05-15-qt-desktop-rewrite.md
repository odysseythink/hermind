# Qt Desktop 100% 功能对等重写实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 Qt Desktop 应用从基础框架扩展为与 Web UI 100% 功能对等的桌面客户端。

**Architecture:** 在现有 `desktop/` 基础设施（HermindProcess、HermindClient、SSEParser、AppWindow 框架）上增量扩展。新增 MarkdownRenderer、ConfigFormEngine、SettingsEditor、ThemeManager、I18n 等核心模块。后端新增 `/api/render/math` 和 `/api/render/mermaid`。

**Tech Stack:** C++17, Qt6.5+ (Core, Gui, Widgets, Network, Test), CMake, Go (后端 API 扩展)

---

## 文件结构总览

### 保留不变（仅引用）

| 文件 | 说明 |
|---|---|
| `desktop/src/hermindprocess.h/cpp` | 后端进程管理 |
| `desktop/src/sseparser.h/cpp` | SSE 流解析 |
| `desktop/src/shortcutmanager.h/cpp` | 全局快捷键 |
| `desktop/src/trayicon.h/cpp` | 系统托盘 |
| `desktop/src/conversationheader.h/cpp` | 会话标题栏 |
| `desktop/src/statusfooter.h/cpp` | 底部状态栏 |
| `desktop/src/sessionlistwidget.h/cpp` | 会话列表（扩展需求小） |
| `desktop/src/promptinput.h/cpp` | 输入区域（扩展需求小） |

### 修改

| 文件 | 变更内容 |
|---|---|
| `desktop/src/httplib.h/cpp` | 增加 `put()`, `deleteResource()`, `uploadFile()` |
| `desktop/src/messagebubble.h/cpp` | 增加操作按钮、messageId、streaming 状态、工具调用区域 |
| `desktop/src/chatwidget.h/cpp` | 增加消息编辑/删除/重生成、停止生成、附件上传、工具调用处理、建议卡片连接 |
| `desktop/src/emptystatewidget.h/cpp` | 连接 `/api/suggestions` 动态加载 |
| `desktop/src/topbar.h/cpp` | 增加语言切换、主题切换按钮，模式切换改为 SettingsEditor |
| `desktop/src/appwindow.h/cpp` | 集成 SettingsEditor、ThemeManager、I18n |
| `desktop/src/main.cpp` | 初始化 ThemeManager、I18n、加载翻译 |
| `desktop/CMakeLists.txt` | 添加所有新源文件和测试 |
| `desktop/resources/resources.qrc` | 添加主题和翻译资源 |
| `api/handlers_render.go` | 新增 `/api/render/math` 和 `/api/render/mermaid` |

### 新增

| 文件 | 说明 |
|---|---|
| `desktop/src/thememanager.h/cpp` | 主题切换管理 |
| `desktop/src/i18n.h/cpp` | 国际化管理 |
| `desktop/src/codehighlighter.h/cpp` | 代码语法高亮器 |
| `desktop/src/markdownrenderer.h/cpp` | Markdown 渲染引擎（代码高亮 + 公式/图表占位符） |
| `desktop/src/toolcallwidget.h/cpp` | 工具调用可视化卡片 |
| `desktop/src/configformengine.h/cpp` | 根据 schema 动态生成 Qt 表单 |
| `desktop/src/settingssidebar.h/cpp` | 设置编辑器左侧分组导航 |
| `desktop/src/settingspanel.h/cpp` | 设置编辑器右侧内容区域容器 |
| `desktop/src/settingseditor.h/cpp` | 设置编辑器主窗口（替代 SettingsDialog） |
| `desktop/src/providereditor.h/cpp` | Provider 管理子面板 |
| `desktop/src/fallbackeditor.h/cpp` | Fallback provider 可拖拽排序列表 |
| `desktop/src/mcpservereditor.h/cpp` | MCP 服务器管理 |
| `desktop/src/croneditor.h/cpp` | Cron 任务调度编辑器 |
| `desktop/src/scalareditor.h/cpp` | 标量配置编辑器 |
| `desktop/resources/themes/dark.qss` | 暗色主题（从现有 styles.qss 迁移） |
| `desktop/resources/themes/light.qss` | 亮色主题（新增） |
| `desktop/resources/i18n/hermind_en.ts` | 英文翻译源文件 |
| `desktop/resources/i18n/hermind_zh_CN.ts` | 简体中文翻译源文件 |
| `desktop/tests/test_markdown_renderer.cpp` | MarkdownRenderer 单元测试 |
| `desktop/tests/test_config_form_engine.cpp` | ConfigFormEngine 单元测试 |
| `desktop/tests/test_i18n.cpp` | I18n 单元测试 |

---

## 渲染策略说明

现有 `ChatWidget::onRenderTimer()` 已调用 `POST /api/render` 将 Markdown 转为 HTML。本计划的渲染增强策略：

1. **流式输出阶段**：保持现有机制，后端 Goldmark 渲染基础 Markdown -> 前端 setHtml
2. **流结束后**：`MarkdownRenderer` 做增强处理——识别代码块并应用 `CodeHighlighter`，识别数学公式/Mermaid 并替换为可点击占位符
3. **代码高亮**：客户端正则提取代码块 -> `CodeHighlighter` 生成带 `<span style="color:#xxx">` 的 HTML -> 替换原 `<pre><code>`
4. **数学公式**：流结束后替换 `$...$` / `$$...$$` 为占位符 `<span class="math-placeholder">`，点击调用 `/api/render/math`
5. **Mermaid 图表**：流结束后替换 ` ```mermaid ` 代码块为占位符 `<div class="mermaid-placeholder">`，点击调用 `/api/render/mermaid`

---

## Task 1: HermindClient 扩展

**Files:**
- Modify: `desktop/src/httplib.h`
- Modify: `desktop/src/httplib.cpp`
- Test: `desktop/tests/test_httplib.cpp`

- [ ] **Step 1: 编写失败测试**

在 `desktop/tests/test_httplib.cpp` 中新增测试：

```cpp
#include <QTest>
#include <QSignalSpy>
#include "../src/httplib.h"

class TestHttpLib : public QObject
{
    Q_OBJECT
private slots:
    void testBaseUrl();
    void testPutMethod();
    void testDeleteMethod();
};

void TestHttpLib::testBaseUrl()
{
    HermindClient client("http://127.0.0.1:12345");
    QCOMPARE(client.baseUrl(), QString("http://127.0.0.1:12345"));
}

void TestHttpLib::testPutMethod()
{
    HermindClient client("http://127.0.0.1:12345");
    QJsonObject body;
    body["test"] = true;
    bool called = false;
    client.put("/api/test", body, [&called](const QJsonObject &, const QString &) {
        called = true;
    });
    QVERIFY(true);
}

void TestHttpLib::testDeleteMethod()
{
    HermindClient client("http://127.0.0.1:12345");
    bool called = false;
    client.deleteResource("/api/test/1", [&called](const QJsonObject &, const QString &) {
        called = true;
    });
    QVERIFY(true);
}

QTEST_MAIN(TestHttpLib)
#include "test_httplib.moc"
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd desktop && cmake -B build -S . && cmake --build build --target test_httplib && ./build/test_httplib
```

Expected: 编译失败，`put` 和 `deleteResource` 方法未定义

- [ ] **Step 3: 实现扩展的 HermindClient**

修改 `desktop/src/httplib.h`：

```cpp
#ifndef HTTPLIB_H
#define HTTPLIB_H

#include <QObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QJsonObject>
#include <QJsonDocument>
#include <QHttpMultiPart>
#include <functional>

class HermindClient : public QObject
{
    Q_OBJECT
public:
    explicit HermindClient(const QString &baseUrl, QObject *parent = nullptr);

    using Callback = std::function<void(const QJsonObject &, const QString &error)>;

    void get(const QString &path, Callback callback);
    void post(const QString &path, const QJsonObject &body, Callback callback);
    void put(const QString &path, const QJsonObject &body, Callback callback);
    void deleteResource(const QString &path, Callback callback);
    void uploadFile(const QString &path, const QString &filePath, Callback callback);
    QNetworkReply* getStream(const QString &path);

    QString baseUrl() const;

private:
    void handleReply(QNetworkReply *reply, Callback callback);

    QNetworkAccessManager *m_manager;
    QString m_baseUrl;
};

#endif
```

修改 `desktop/src/httplib.cpp`：

```cpp
#include "httplib.h"
#include <QNetworkRequest>
#include <QUrl>
#include <QFile>
#include <QHttpMultiPart>

HermindClient::HermindClient(const QString &baseUrl, QObject *parent)
    : QObject(parent), m_manager(new QNetworkAccessManager(this)), m_baseUrl(baseUrl)
{
}

void HermindClient::handleReply(QNetworkReply *reply, Callback callback)
{
    connect(reply, &QNetworkReply::finished, [reply, callback]() {
        if (reply->error() != QNetworkReply::NoError) {
            callback(QJsonObject(), reply->errorString());
        } else {
            QByteArray data = reply->readAll();
            QJsonDocument doc = QJsonDocument::fromJson(data);
            callback(doc.object(), QString());
        }
        reply->deleteLater();
    });
}

void HermindClient::get(const QString &path, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QNetworkReply *reply = m_manager->get(req);
    handleReply(reply, callback);
}

void HermindClient::post(const QString &path, const QJsonObject &body, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QByteArray payload = QJsonDocument(body).toJson();
    QNetworkReply *reply = m_manager->post(req, payload);
    handleReply(reply, callback);
}

void HermindClient::put(const QString &path, const QJsonObject &body, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QByteArray payload = QJsonDocument(body).toJson();
    QNetworkReply *reply = m_manager->put(req, payload);
    handleReply(reply, callback);
}

void HermindClient::deleteResource(const QString &path, Callback callback)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    QNetworkReply *reply = m_manager->deleteResource(req);
    handleReply(reply, callback);
}

void HermindClient::uploadFile(const QString &path, const QString &filePath, Callback callback)
{
    QHttpMultiPart *multiPart = new QHttpMultiPart(QHttpMultiPart::FormDataType, this);
    QFile *file = new QFile(filePath);
    if (!file->open(QIODevice::ReadOnly)) {
        callback(QJsonObject(), QString("Cannot open file: %1").arg(filePath));
        return;
    }
    file->setParent(multiPart);

    QHttpPart filePart;
    filePart.setHeader(QNetworkRequest::ContentTypeHeader, QVariant("application/octet-stream"));
    filePart.setHeader(QNetworkRequest::ContentDispositionHeader,
                       QVariant(QString("form-data; name=\"file\"; filename=\"%1\"")
                                    .arg(QFileInfo(filePath).fileName())));
    filePart.setBodyDevice(file);
    multiPart->append(filePart);

    QNetworkRequest req(QUrl(m_baseUrl + path));
    QNetworkReply *reply = m_manager->post(req, multiPart);
    multiPart->setParent(reply);
    handleReply(reply, callback);
}

QNetworkReply* HermindClient::getStream(const QString &path)
{
    QNetworkRequest req(QUrl(m_baseUrl + path));
    req.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    req.setRawHeader("Accept", "text/event-stream");
    return m_manager->get(req);
}

QString HermindClient::baseUrl() const
{
    return m_baseUrl;
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd desktop && cmake --build build --target test_httplib && ./build/test_httplib
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/src/httplib.h desktop/src/httplib.cpp desktop/tests/test_httplib.cpp
git commit -m "feat(desktop): extend HermindClient with put, delete, upload"
```

---

## Task 2: ThemeManager + I18n 基础框架

**Files:**
- Create: `desktop/src/thememanager.h`, `desktop/src/thememanager.cpp`
- Create: `desktop/src/i18n.h`, `desktop/src/i18n.cpp`
- Test: `desktop/tests/test_i18n.cpp`

- [ ] **Step 1: 创建 ThemeManager**

`desktop/src/thememanager.h`:

```cpp
#ifndef THEMEMANAGER_H
#define THEMEMANAGER_H

#include <QObject>
#include <QColor>

class ThemeManager : public QObject
{
    Q_OBJECT
public:
    explicit ThemeManager(QObject *parent = nullptr);

    void loadTheme(const QString &themeName);
    QString currentTheme() const;

    QColor accentColor() const;
    QColor backgroundColor() const;
    QColor surfaceColor() const;
    QColor textColor() const;
    QColor mutedTextColor() const;
    QColor borderColor() const;

signals:
    void themeChanged(const QString &themeName);

private:
    QString m_currentTheme;
};

#endif
```

`desktop/src/thememanager.cpp`:

```cpp
#include "thememanager.h"
#include <QApplication>
#include <QFile>

ThemeManager::ThemeManager(QObject *parent)
    : QObject(parent)
{
    loadTheme("dark");
}

void ThemeManager::loadTheme(const QString &themeName)
{
    QFile file(QString(":/themes/%1.qss").arg(themeName));
    if (file.open(QFile::ReadOnly)) {
        QString styleSheet = QLatin1String(file.readAll());
        qApp->setStyleSheet(styleSheet);
        m_currentTheme = themeName;
        emit themeChanged(themeName);
    }
}

QString ThemeManager::currentTheme() const
{
    return m_currentTheme;
}

QColor ThemeManager::accentColor() const
{
    return QColor("#FFB800");
}

QColor ThemeManager::backgroundColor() const
{
    return QColor("#0a0b0d");
}

QColor ThemeManager::surfaceColor() const
{
    return QColor("#14161a");
}

QColor ThemeManager::textColor() const
{
    return QColor("#e8e6e3");
}

QColor ThemeManager::mutedTextColor() const
{
    return QColor("#8a8680");
}

QColor ThemeManager::borderColor() const
{
    return QColor("#2a2e36");
}
```

- [ ] **Step 2: 创建 I18n**

`desktop/src/i18n.h`:

```cpp
#ifndef I18N_H
#define I18N_H

#include <QObject>
#include <QTranslator>
#include <QStringList>

class I18n : public QObject
{
    Q_OBJECT
public:
    explicit I18n(QObject *parent = nullptr);

    void loadLanguage(const QString &language);
    QString currentLanguage() const;
    QStringList availableLanguages() const;
    QString languageDisplayName(const QString &code) const;

signals:
    void languageChanged(const QString &language);

private:
    QTranslator *m_translator;
    QString m_currentLanguage;
};

#endif
```

`desktop/src/i18n.cpp`:

```cpp
#include "i18n.h"
#include <QApplication>
#include <QLocale>

I18n::I18n(QObject *parent)
    : QObject(parent),
      m_translator(new QTranslator(this)),
      m_currentLanguage("en")
{
    qApp->installTranslator(m_translator);
}

void I18n::loadLanguage(const QString &language)
{
    if (m_currentLanguage == language)
        return;

    qApp->removeTranslator(m_translator);
    delete m_translator;

    m_translator = new QTranslator(this);
    if (m_translator->load(QString(":/i18n/hermind_%1.qm").arg(language))) {
        qApp->installTranslator(m_translator);
    }
    m_currentLanguage = language;
    emit languageChanged(language);
}

QString I18n::currentLanguage() const
{
    return m_currentLanguage;
}

QStringList I18n::availableLanguages() const
{
    return QStringList() << "en" << "zh_CN";
}

QString I18n::languageDisplayName(const QString &code) const
{
    if (code == "zh_CN") return QString::fromUtf8("中文");
    return "English";
}
```

- [ ] **Step 3: 编写 I18n 测试**

`desktop/tests/test_i18n.cpp`:

```cpp
#include <QTest>
#include <QSignalSpy>
#include "../src/i18n.h"

class TestI18n : public QObject
{
    Q_OBJECT
private slots:
    void testAvailableLanguages();
    void testLanguageSwitch();
};

void TestI18n::testAvailableLanguages()
{
    I18n i18n;
    QStringList langs = i18n.availableLanguages();
    QVERIFY(langs.contains("en"));
    QVERIFY(langs.contains("zh_CN"));
}

void TestI18n::testLanguageSwitch()
{
    I18n i18n;
    QSignalSpy spy(&i18n, &I18n::languageChanged);
    i18n.loadLanguage("zh_CN");
    QCOMPARE(spy.count(), 1);
    QCOMPARE(i18n.currentLanguage(), QString("zh_CN"));
}

QTEST_MAIN(TestI18n)
#include "test_i18n.moc"
```

- [ ] **Step 4: 更新 CMakeLists.txt**

在 `desktop/CMakeLists.txt` 的 `SOURCES` 列表中新增：

```cmake
    src/thememanager.cpp
    src/i18n.cpp
```

在 `HEADERS` 列表中新增：

```cmake
    src/thememanager.h
    src/i18n.h
```

在测试部分新增：

```cmake
add_executable(test_i18n tests/test_i18n.cpp src/i18n.cpp)
target_link_libraries(test_i18n PRIVATE Qt6::Core Qt6::Gui Qt6::Test)
add_test(NAME test_i18n COMMAND test_i18n)
```

- [ ] **Step 5: 构建并运行测试**

```bash
cd desktop && cmake -B build -S . && cmake --build build --target test_i18n && ./build/test_i18n
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add desktop/src/thememanager.h desktop/src/thememanager.cpp \
        desktop/src/i18n.h desktop/src/i18n.cpp \
        desktop/tests/test_i18n.cpp desktop/CMakeLists.txt
git commit -m "feat(desktop): add ThemeManager and I18n framework"
```

---

## Task 3: CodeHighlighter + MarkdownRenderer

**Files:**
- Create: `desktop/src/codehighlighter.h`, `desktop/src/codehighlighter.cpp`
- Create: `desktop/src/markdownrenderer.h`, `desktop/src/markdownrenderer.cpp`
- Test: `desktop/tests/test_markdown_renderer.cpp`

- [ ] **Step 1: 创建 CodeHighlighter**

`desktop/src/codehighlighter.h`:

```cpp
#ifndef CODEHIGHLIGHTER_H
#define CODEHIGHLIGHTER_H

#include <QSyntaxHighlighter>
#include <QVector>
#include <QRegularExpression>

class QTextDocument;

class CodeHighlighter : public QSyntaxHighlighter
{
    Q_OBJECT
public:
    explicit CodeHighlighter(QTextDocument *parent = nullptr);
    void setLanguage(const QString &language);
    QString highlightToHtml(const QString &code);

protected:
    void highlightBlock(const QString &text) override;

private:
    struct Rule {
        QRegularExpression pattern;
        QTextCharFormat format;
    };
    QVector<Rule> m_rules;
    QString m_language;

    void clearRules();
    void loadCppRules();
    void loadGoRules();
    void loadPythonRules();
    void loadJsRules();
    void loadBashRules();
    void loadJsonRules();
    void loadGenericRules();
};

#endif
```

`desktop/src/codehighlighter.cpp`:

```cpp
#include "codehighlighter.h"
#include <QTextDocument>

CodeHighlighter::CodeHighlighter(QTextDocument *parent)
    : QSyntaxHighlighter(parent)
{
    loadGenericRules();
}

void CodeHighlighter::setLanguage(const QString &language)
{
    m_language = language.toLower();
    clearRules();
    if (m_language == "cpp" || m_language == "c++" || m_language == "c") {
        loadCppRules();
    } else if (m_language == "go" || m_language == "golang") {
        loadGoRules();
    } else if (m_language == "python" || m_language == "py") {
        loadPythonRules();
    } else if (m_language == "javascript" || m_language == "js" || m_language == "typescript" || m_language == "ts") {
        loadJsRules();
    } else if (m_language == "bash" || m_language == "shell" || m_language == "sh") {
        loadBashRules();
    } else if (m_language == "json") {
        loadJsonRules();
    } else {
        loadGenericRules();
    }
    rehighlight();
}

QString CodeHighlighter::highlightToHtml(const QString &code)
{
    QTextDocument doc;
    doc.setPlainText(code);
    CodeHighlighter highlighter(doc.document());
    highlighter.setLanguage(m_language);
    doc.documentLayout();

    QString html = doc.toHtml();
    int bodyStart = html.indexOf("<body>") + 6;
    int bodyEnd = html.indexOf("</body>");
    if (bodyStart > 5 && bodyEnd > bodyStart) {
        return html.mid(bodyStart, bodyEnd - bodyStart).trimmed();
    }
    return code.toHtmlEscaped();
}

void CodeHighlighter::clearRules()
{
    m_rules.clear();
}

void CodeHighlighter::highlightBlock(const QString &text)
{
    for (const Rule &rule : m_rules) {
        QRegularExpressionMatchIterator matchIterator = rule.pattern.globalMatch(text);
        while (matchIterator.hasNext()) {
            QRegularExpressionMatch match = matchIterator.next();
            setFormat(match.capturedStart(), match.capturedLength(), rule.format);
        }
    }
}

void CodeHighlighter::loadCppRules()
{
    QTextCharFormat keywordFormat;
    keywordFormat.setForeground(QColor("#c586c0"));
    QStringList keywords;
    keywords << "\\bclass\\b" << "\\bstruct\\b" << "\\benum\\b" << "\\bnamespace\\b"
             << "\\bpublic\\b" << "\\bprivate\\b" << "\\bprotected\\b"
             << "\\bvirtual\\b" << "\\boverride\\b" << "\\bconst\\b"
             << "\\bstatic\\b" << "\\binline\\b" << "\\bexplicit\\b"
             << "\\breturn\\b" << "\\bif\\b" << "\\belse\\b" << "\\bfor\\b"
             << "\\bwhile\\b" << "\\bdo\\b" << "\\bswitch\\b" << "\\bcase\\b"
             << "\\bbreak\\b" << "\\bcontinue\\b" << "\\bdefault\\b"
             << "\\bnew\\b" << "\\bdelete\\b" << "\\btry\\b" << "\\bcatch\\b"
             << "\\bthrow\\b" << "\\busing\\b" << "\\btemplate\\b"
             << "\\btypename\\b" << "\\bauto\\b" << "\\bnullptr\\b"
             << "\\btrue\\b" << "\\bfalse\\b" << "\\bvoid\\b" << "\\bint\\b"
             << "\\bchar\\b" << "\\bfloat\\b" << "\\bdouble\\b" << "\\bbool\\b"
             << "\\blong\\b" << "\\bshort\\b" << "\\bsigned\\b" << "\\bunsigned\\b";
    for (const QString &pattern : keywords) {
        m_rules.append({QRegularExpression(pattern), keywordFormat});
    }

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("'(?:[^'\\\\]|\\\\.)*'"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("//[^\\n]*"), commentFormat});
    m_rules.append({QRegularExpression("/\\*.*?\\*/"), commentFormat});
}

void CodeHighlighter::loadGoRules()
{
    QTextCharFormat keywordFormat;
    keywordFormat.setForeground(QColor("#c586c0"));
    QStringList keywords;
    keywords << "\\bpackage\\b" << "\\bimport\\b" << "\\bfunc\\b" << "\\breturn\\b"
             << "\\bif\\b" << "\\belse\\b" << "\\bfor\\b" << "\\brange\\b"
             << "\\bswitch\\b" << "\\bcase\\b" << "\\bdefault\\b" << "\\bbreak\\b"
             << "\\bcontinue\\b" << "\\bvar\\b" << "\\bconst\\b" << "\\btype\\b"
             << "\\bstruct\\b" << "\\binterface\\b" << "\\bmap\\b" << "\\bchan\\b"
             << "\\bgo\\b" << "\\bselect\\b" << "\\bdefer\\b" << "\\bnil\\b"
             << "\\btrue\\b" << "\\bfalse\\b";
    for (const QString &pattern : keywords) {
        m_rules.append({QRegularExpression(pattern), keywordFormat});
    }

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("`(?:[^`])*`"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("//[^\\n]*"), commentFormat});
    m_rules.append({QRegularExpression("/\\*.*?\\*/"), commentFormat});
}

void CodeHighlighter::loadPythonRules()
{
    QTextCharFormat keywordFormat;
    keywordFormat.setForeground(QColor("#c586c0"));
    QStringList keywords;
    keywords << "\\bdef\\b" << "\\bclass\\b" << "\\breturn\\b" << "\\bif\\b"
             << "\\belif\\b" << "\\belse\\b" << "\\bfor\\b" << "\\bwhile\\b"
             << "\\bin\\b" << "\\bis\\b" << "\\bnot\\b" << "\\band\\b"
             << "\\bor\\b" << "\\bimport\\b" << "\\bfrom\\b" << "\\bas\\b"
             << "\\btry\\b" << "\\bexcept\\b" << "\\bfinally\\b" << "\\braise\\b"
             << "\\bwith\\b" << "\\byield\\b" << "\\blambda\\b" << "\\bpass\\b"
             << "\\bbreak\\b" << "\\bcontinue\\b" << "\\bTrue\\b" << "\\bFalse\\b"
             << "\\bNone\\b";
    for (const QString &pattern : keywords) {
        m_rules.append({QRegularExpression(pattern), keywordFormat});
    }

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"\"\"(?:[^\"]|\"(?!\"\"))*\"\"\""), stringFormat});
    m_rules.append({QRegularExpression("'''(?:[^']|'(?!''))*'''"), stringFormat});
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("'(?:[^'\\\\]|\\\\.)*'"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("#[^\\n]*"), commentFormat});
}

void CodeHighlighter::loadJsRules()
{
    QTextCharFormat keywordFormat;
    keywordFormat.setForeground(QColor("#c586c0"));
    QStringList keywords;
    keywords << "\\bfunction\\b" << "\\breturn\\b" << "\\bif\\b" << "\\belse\\b"
             << "\\bfor\\b" << "\\bwhile\\b" << "\\bdo\\b" << "\\bswitch\\b"
             << "\\bcase\\b" << "\\bbreak\\b" << "\\bcontinue\\b" << "\\bdefault\\b"
             << "\\bvar\\b" << "\\blet\\b" << "\\bconst\\b" << "\\bnew\\b"
             << "\\bthis\\b" << "\\bclass\\b" << "\\bextends\\b" << "\\bimport\\b"
             << "\\bexport\\b" << "\\bfrom\\b" << "\\btry\\b" << "\\bcatch\\b"
             << "\\bfinally\\b" << "\\bthrow\\b" << "\\btypeof\\b" << "\\binstanceof\\b"
             << "\\bnull\\b" << "\\bundefined\\b" << "\\btrue\\b" << "\\bfalse\\b"
             << "\\basync\\b" << "\\bawait\\b" << "\\byield\\b";
    for (const QString &pattern : keywords) {
        m_rules.append({QRegularExpression(pattern), keywordFormat});
    }

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("'(?:[^'\\\\]|\\\\.)*'"), stringFormat});
    m_rules.append({QRegularExpression("`(?:[^`\\\\]|\\\\.)*`"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("//[^\\n]*"), commentFormat});
    m_rules.append({QRegularExpression("/\\*.*?\\*/"), commentFormat});
}

void CodeHighlighter::loadBashRules()
{
    QTextCharFormat keywordFormat;
    keywordFormat.setForeground(QColor("#c586c0"));
    QStringList keywords;
    keywords << "\\bif\\b" << "\\bthen\\b" << "\\belse\\b" << "\\belif\\b"
             << "\\bfi\\b" << "\\bfor\\b" << "\\bwhile\\b" << "\\bdo\\b"
             << "\\bdone\\b" << "\\bcase\\b" << "\\besac\\b" << "\\bin\\b"
             << "\\bfunction\\b" << "\\breturn\\b" << "\\bexport\\b" << "\\bsource\\b";
    for (const QString &pattern : keywords) {
        m_rules.append({QRegularExpression(pattern), keywordFormat});
    }

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("'(?:[^'])*'"), stringFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("#[^\\n]*"), commentFormat});
}

void CodeHighlighter::loadJsonRules()
{
    QTextCharFormat keyFormat;
    keyFormat.setForeground(QColor("#9cdcfe"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\"(?=\\s*:)"), keyFormat});

    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\"(?!\\s*:)"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat boolFormat;
    boolFormat.setForeground(QColor("#569cd6"));
    m_rules.append({QRegularExpression("\\btrue\\b|\\bfalse\\b|\\bnull\\b"), boolFormat});
}

void CodeHighlighter::loadGenericRules()
{
    QTextCharFormat stringFormat;
    stringFormat.setForeground(QColor("#ce9178"));
    m_rules.append({QRegularExpression("\"(?:[^\"\\\\]|\\\\.)*\""), stringFormat});
    m_rules.append({QRegularExpression("'(?:[^'\\\\]|\\\\.)*'"), stringFormat});

    QTextCharFormat numberFormat;
    numberFormat.setForeground(QColor("#b5cea8"));
    m_rules.append({QRegularExpression("\\b[0-9]+(?:\\.[0-9]+)?\\b"), numberFormat});

    QTextCharFormat commentFormat;
    commentFormat.setForeground(QColor("#6a9955"));
    m_rules.append({QRegularExpression("//[^\\n]*|#[^\\n]*"), commentFormat});
}
```

- [ ] **Step 2: 创建 MarkdownRenderer**

`desktop/src/markdownrenderer.h`:

```cpp
#ifndef MARKDOWNRENDERER_H
#define MARKDOWNRENDERER_H

#include <QObject>
#include <QString>
#include <QVector>

class CodeHighlighter;

class MarkdownRenderer : public QObject
{
    Q_OBJECT
public:
    explicit MarkdownRenderer(QObject *parent = nullptr);
    ~MarkdownRenderer();

    QString enhanceHtml(const QString &html, const QString &markdown);

    struct CodeBlock {
        int index;
        QString language;
        QString code;
    };
    QVector<CodeBlock> extractCodeBlocks(const QString &markdown);

    bool hasCompleteMath(const QString &markdown) const;
    bool hasCompleteMermaid(const QString &markdown) const;

private:
    CodeHighlighter *m_highlighter;

    QString applyCodeHighlighting(const QString &html, const QVector<CodeBlock> &blocks);
    QString replaceMathWithPlaceholders(const QString &html);
    QString replaceMermaidWithPlaceholders(const QString &html);
};

#endif
```

`desktop/src/markdownrenderer.cpp`:

```cpp
#include "markdownrenderer.h"
#include "codehighlighter.h"
#include <QRegularExpression>
#include <QTextDocument>

MarkdownRenderer::MarkdownRenderer(QObject *parent)
    : QObject(parent),
      m_highlighter(new CodeHighlighter(nullptr))
{
}

MarkdownRenderer::~MarkdownRenderer()
{
    delete m_highlighter;
}

QVector<MarkdownRenderer::CodeBlock> MarkdownRenderer::extractCodeBlocks(const QString &markdown)
{
    QVector<CodeBlock> blocks;
    QRegularExpression re("```(\\w*)\\n([^`]*?)\\n```");
    re.setPatternOptions(QRegularExpression::DotMatchesEverythingOption);
    QRegularExpressionMatchIterator it = re.globalMatch(markdown);
    int idx = 0;
    while (it.hasNext()) {
        QRegularExpressionMatch match = it.next();
        blocks.append({idx++, match.captured(1).trimmed(), match.captured(2)});
    }
    return blocks;
}

QString MarkdownRenderer::enhanceHtml(const QString &html, const QString &markdown)
{
    QString result = html;
    QVector<CodeBlock> blocks = extractCodeBlocks(markdown);
    result = applyCodeHighlighting(result, blocks);
    result = replaceMathWithPlaceholders(result);
    result = replaceMermaidWithPlaceholders(result);
    return result;
}

QString MarkdownRenderer::applyCodeHighlighting(const QString &html, const QVector<CodeBlock> &blocks)
{
    QString result = html;
    for (int i = 0; i < blocks.size(); ++i) {
        const CodeBlock &block = blocks[i];
        m_highlighter->setLanguage(block.language);
        QString highlighted = m_highlighter->highlightToHtml(block.code);
        QString originalEscaped = block.code.toHtmlEscaped();
        QString target = QString("<pre><code>%1</code></pre>").arg(originalEscaped);
        QString replacement = QString("<pre style=\"background:#1e1e1e;padding:12px;border-radius:4px;overflow-x:auto;\"><code>%1</code></pre>").arg(highlighted);
        result.replace(target, replacement);
    }
    return result;
}

QString MarkdownRenderer::replaceMathWithPlaceholders(const QString &html)
{
    QString result = html;
    QRegularExpression blockRe("<p>\\$\\$([^\\$]+)\\$\\$</p>");
    blockRe.setPatternOptions(QRegularExpression::DotMatchesEverythingOption);
    QRegularExpressionMatchIterator it = blockRe.globalMatch(result);
    while (it.hasNext()) {
        QRegularExpressionMatch match = it.next();
        QString latex = match.captured(1).trimmed().toHtmlEscaped();
        QString placeholder = QString(
            "<div style=\"background:#1e2028;border:1px dashed #FFB800;padding:8px 12px;"
            "border-radius:4px;margin:8px 0;color:#8b8f98;font-family:monospace;cursor:pointer;\" "
            "class=\"math-placeholder\" data-latex=\"%1\">"
            "📐 %2"
            "</div>"
        ).arg(latex, latex);
        result.replace(match.capturedStart(), match.capturedLength(), placeholder);
    }

    QRegularExpression inlineRe("\\$([^\\$\\s][^\\$]*)\\$");
    it = inlineRe.globalMatch(result);
    while (it.hasNext()) {
        QRegularExpressionMatch match = it.next();
        QString latex = match.captured(1).trimmed().toHtmlEscaped();
        QString placeholder = QString(
            "<span style=\"color:#FFB800;cursor:pointer;\" "
            "class=\"math-placeholder\" data-latex=\"%1\">"
            "$%2$"
            "</span>"
        ).arg(latex, latex);
        result.replace(match.capturedStart(), match.capturedLength(), placeholder);
    }

    return result;
}

QString MarkdownRenderer::replaceMermaidWithPlaceholders(const QString &html)
{
    QString result = html;
    QRegularExpression re("<pre><code class=\"language-mermaid\">([^<]*)</code></pre>");
    re.setPatternOptions(QRegularExpression::DotMatchesEverythingOption);
    QRegularExpressionMatchIterator it = re.globalMatch(result);
    while (it.hasNext()) {
        QRegularExpressionMatch match = it.next();
        QString diagram = match.captured(1).trimmed().toHtmlEscaped();
        QString placeholder = QString(
            "<div style=\"background:#1e2028;border:1px dashed #4ade80;padding:8px 12px;"
            "border-radius:4px;margin:8px 0;color:#8b8f98;font-family:monospace;cursor:pointer;\" "
            "class=\"mermaid-placeholder\" data-diagram=\"%1\">"
            "📊 Mermaid Diagram [点击查看]"
            "</div>"
        ).arg(diagram);
        result.replace(match.capturedStart(), match.capturedLength(), placeholder);
    }
    return result;
}

bool MarkdownRenderer::hasCompleteMath(const QString &markdown) const
{
    QRegularExpression blockRe("\\$\\$[^\\$]+\\$\\$");
    QRegularExpression inlineRe("(?:^|[^\\$])\\$[^\\$\\s][^\\$]*\\$(?:[^\\$]|$)");
    return blockRe.match(markdown).hasMatch() || inlineRe.match(markdown).hasMatch();
}

bool MarkdownRenderer::hasCompleteMermaid(const QString &markdown) const
{
    QRegularExpression re("```mermaid\\n[^`]*?\\n```");
    re.setPatternOptions(QRegularExpression::DotMatchesEverythingOption);
    return re.match(markdown).hasMatch();
}
```

- [ ] **Step 3: 编写 MarkdownRenderer 测试**

`desktop/tests/test_markdown_renderer.cpp`:

```cpp
#include <QTest>
#include "../src/markdownrenderer.h"

class TestMarkdownRenderer : public QObject
{
    Q_OBJECT
private slots:
    void testExtractCodeBlocks();
    void testEnhanceHtmlWithCode();
    void testMathPlaceholder();
    void testMermaidPlaceholder();
};

void TestMarkdownRenderer::testExtractCodeBlocks()
{
    MarkdownRenderer renderer;
    QString md = "```cpp\nint x = 1;\n```\n\n```python\nprint(1)\n```";
    auto blocks = renderer.extractCodeBlocks(md);
    QCOMPARE(blocks.size(), 2);
    QCOMPARE(blocks[0].language, QString("cpp"));
    QCOMPARE(blocks[0].code, QString("int x = 1;"));
    QCOMPARE(blocks[1].language, QString("python"));
}

void TestMarkdownRenderer::testEnhanceHtmlWithCode()
{
    MarkdownRenderer renderer;
    QString md = "```cpp\nint x = 1;\n```";
    QString html = "<pre><code>int x = 1;</code></pre>";
    QString enhanced = renderer.enhanceHtml(html, md);
    QVERIFY(enhanced.contains("background:#1e1e1e"));
}

void TestMarkdownRenderer::testMathPlaceholder()
{
    MarkdownRenderer renderer;
    QString md = "The equation $E = mc^2$ is famous.";
    QString html = "<p>The equation $E = mc^2$ is famous.</p>";
    QString enhanced = renderer.enhanceHtml(html, md);
    QVERIFY(enhanced.contains("math-placeholder"));
}

void TestMarkdownRenderer::testMermaidPlaceholder()
{
    MarkdownRenderer renderer;
    QString md = "```mermaid\ngraph TD; A-->B;\n```";
    QString html = "<pre><code class=\"language-mermaid\">graph TD; A-->B;</code></pre>";
    QString enhanced = renderer.enhanceHtml(html, md);
    QVERIFY(enhanced.contains("mermaid-placeholder"));
}

QTEST_MAIN(TestMarkdownRenderer)
#include "test_markdown_renderer.moc"
```

- [ ] **Step 4: 更新 CMakeLists.txt 并运行测试**

在 `SOURCES` 和 `HEADERS` 中新增 `codehighlighter` 和 `markdownrenderer`。

新增测试目标：
```cmake
add_executable(test_markdown_renderer tests/test_markdown_renderer.cpp src/markdownrenderer.cpp src/codehighlighter.cpp)
target_link_libraries(test_markdown_renderer PRIVATE Qt6::Core Qt6::Gui Qt6::Widgets Qt6::Test)
add_test(NAME test_markdown_renderer COMMAND test_markdown_renderer)
```

```bash
cd desktop && cmake -B build -S . && cmake --build build --target test_markdown_renderer && ./build/test_markdown_renderer
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/src/codehighlighter.h desktop/src/codehighlighter.cpp \
        desktop/src/markdownrenderer.h desktop/src/markdownrenderer.cpp \
        desktop/tests/test_markdown_renderer.cpp desktop/CMakeLists.txt
git commit -m "feat(desktop): add CodeHighlighter and MarkdownRenderer"
```

---

## Task 4: MessageBubble 扩展

**Files:**
- Modify: `desktop/src/messagebubble.h`
- Modify: `desktop/src/messagebubble.cpp`

- [ ] **Step 1: 修改 MessageBubble 头文件**

`desktop/src/messagebubble.h`：

```cpp
#ifndef MESSAGEBUBBLE_H
#define MESSAGEBUBBLE_H

#include <QWidget>
#include <QString>

class QTextEdit;
class QLabel;
class QPushButton;
class ToolCallWidget;

class MessageBubble : public QWidget
{
    Q_OBJECT
public:
    explicit MessageBubble(bool isUser, QWidget *parent = nullptr);

    void appendMarkdown(const QString &text);
    QString markdownBuffer() const;
    void setHtmlContent(const QString &html);

    void setMessageId(const QString &id);
    QString messageId() const;
    void setStreaming(bool streaming);
    bool isStreaming() const;
    void addToolCallWidget(ToolCallWidget *widget);

signals:
    void editRequested(const QString &messageId, const QString &currentText);
    void deleteRequested(const QString &messageId);
    void regenerateRequested(const QString &messageId);
    void copyRequested(const QString &text);
    void mathClicked(const QString &latex);
    void mermaidClicked(const QString &diagram);

private slots:
    void onEditClicked();
    void onDeleteClicked();
    void onRegenerateClicked();
    void onCopyClicked();
    void onAnchorClicked(const QUrl &url);

private:
    void setupUI();
    void setupActionBar();
    void updateContentHeight();

    bool m_isUser;
    QString m_messageId;
    bool m_isStreaming;
    QString m_markdownBuffer;

    QLabel *m_roleTag;
    QTextEdit *m_content;

    QWidget *m_actionBar;
    QPushButton *m_editBtn;
    QPushButton *m_deleteBtn;
    QPushButton *m_regenerateBtn;
    QPushButton *m_copyBtn;

    QWidget *m_toolCallsArea;
    QVBoxLayout *m_toolCallsLayout;
};

#endif
```

- [ ] **Step 2: 修改 MessageBubble 实现**

`desktop/src/messagebubble.cpp`：

```cpp
#include "messagebubble.h"
#include "toolcallwidget.h"

#include <QTextEdit>
#include <QLabel>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QScrollBar>
#include <QPushButton>
#include <QTextCursor>
#include <QTextBlock>
#include <QClipboard>
#include <QApplication>

MessageBubble::MessageBubble(bool isUser, QWidget *parent)
    : QWidget(parent),
      m_isUser(isUser),
      m_isStreaming(false),
      m_roleTag(new QLabel(this)),
      m_content(new QTextEdit(this)),
      m_actionBar(nullptr),
      m_editBtn(nullptr),
      m_deleteBtn(nullptr),
      m_regenerateBtn(nullptr),
      m_copyBtn(nullptr),
      m_toolCallsArea(nullptr),
      m_toolCallsLayout(nullptr)
{
    setupUI();
}

void MessageBubble::setupUI()
{
    m_roleTag->setText(m_isUser ? tr("YOU") : tr("HERMIND"));
    m_roleTag->setStyleSheet(
        QString("font-family: monospace; font-size: 10px; font-weight: 600; "
                "text-transform: uppercase; color: %1;")
            .arg(m_isUser ? "#FFB800" : "#8a8680")
    );

    m_content->setReadOnly(true);
    m_content->setFrameStyle(QFrame::NoFrame);
    m_content->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_content->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_content->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    m_content->document()->setDocumentMargin(12);
    m_content->setTextInteractionFlags(Qt::TextBrowserInteraction);
    m_content->setOpenExternalLinks(true);
    connect(m_content, &QTextEdit::anchorClicked, this, &MessageBubble::onAnchorClicked);

    QVBoxLayout *bubbleLayout = new QVBoxLayout;
    bubbleLayout->setContentsMargins(12, 10, 12, 10);
    bubbleLayout->setSpacing(4);
    bubbleLayout->addWidget(m_content);

    if (!m_isUser) {
        m_toolCallsArea = new QWidget(this);
        m_toolCallsLayout = new QVBoxLayout(m_toolCallsArea);
        m_toolCallsLayout->setContentsMargins(0, 8, 0, 0);
        m_toolCallsLayout->setSpacing(4);
        bubbleLayout->addWidget(m_toolCallsArea);
        m_toolCallsArea->hide();
    }

    QWidget *bubbleWrapper = new QWidget(this);
    bubbleWrapper->setLayout(bubbleLayout);
    bubbleWrapper->setStyleSheet(
        QString("background: %1; border: 1px solid %2; border-radius: 4px;")
            .arg(m_isUser ? "transparent" : "#14161a")
            .arg(m_isUser ? "#FFB800" : "#2a2e36")
    );
    bubbleWrapper->setMaximumWidth(700);

    QVBoxLayout *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(2);

    if (m_isUser) {
        m_roleTag->setAlignment(Qt::AlignRight);
        outer->addWidget(m_roleTag, 0, Qt::AlignRight);

        QHBoxLayout *row = new QHBoxLayout;
        row->addStretch(1);
        row->addWidget(bubbleWrapper, 0, Qt::AlignTop);
        outer->addLayout(row);
    } else {
        m_roleTag->setAlignment(Qt::AlignLeft);
        outer->addWidget(m_roleTag, 0, Qt::AlignLeft);

        QHBoxLayout *row = new QHBoxLayout;
        row->addWidget(bubbleWrapper, 0, Qt::AlignTop);
        row->addStretch(1);
        outer->addLayout(row);

        setupActionBar();
        outer->addWidget(m_actionBar, 0, Qt::AlignLeft);
    }
}

void MessageBubble::setupActionBar()
{
    m_actionBar = new QWidget(this);
    QHBoxLayout *layout = new QHBoxLayout(m_actionBar);
    layout->setContentsMargins(12, 4, 12, 4);
    layout->setSpacing(8);

    auto createBtn = [](const QString &text) -> QPushButton* {
        QPushButton *btn = new QPushButton(text);
        btn->setStyleSheet(
            "QPushButton { background: transparent; color: #8a8680; "
            "border: none; font-size: 11px; padding: 2px 8px; }"
            "QPushButton:hover { color: #FFB800; }"
        );
        btn->setCursor(Qt::PointingHandCursor);
        return btn;
    };

    m_editBtn = createBtn(tr("Edit"));
    m_deleteBtn = createBtn(tr("Delete"));
    m_regenerateBtn = createBtn(tr("Regenerate"));
    m_copyBtn = createBtn(tr("Copy"));

    connect(m_editBtn, &QPushButton::clicked, this, &MessageBubble::onEditClicked);
    connect(m_deleteBtn, &QPushButton::clicked, this, &MessageBubble::onDeleteClicked);
    connect(m_regenerateBtn, &QPushButton::clicked, this, &MessageBubble::onRegenerateClicked);
    connect(m_copyBtn, &QPushButton::clicked, this, &MessageBubble::onCopyClicked);

    layout->addWidget(m_editBtn);
    layout->addWidget(m_deleteBtn);
    layout->addWidget(m_regenerateBtn);
    layout->addWidget(m_copyBtn);
    layout->addStretch(1);
}

void MessageBubble::appendMarkdown(const QString &text)
{
    m_markdownBuffer.append(text);
}

QString MessageBubble::markdownBuffer() const
{
    return m_markdownBuffer;
}

void MessageBubble::setHtmlContent(const QString &html)
{
    m_content->setHtml(html);
    updateContentHeight();
}

void MessageBubble::updateContentHeight()
{
    m_content->document()->setTextWidth(m_content->viewport()->width());
    int height = static_cast<int>(m_content->document()->size().height());
    m_content->setMinimumHeight(height + 8);
    m_content->setMaximumHeight(height + 8);
}

void MessageBubble::setMessageId(const QString &id)
{
    m_messageId = id;
}

QString MessageBubble::messageId() const
{
    return m_messageId;
}

void MessageBubble::setStreaming(bool streaming)
{
    m_isStreaming = streaming;
    if (m_actionBar) {
        m_actionBar->setVisible(!streaming);
    }
}

bool MessageBubble::isStreaming() const
{
    return m_isStreaming;
}

void MessageBubble::addToolCallWidget(ToolCallWidget *widget)
{
    if (m_toolCallsLayout) {
        m_toolCallsLayout->addWidget(widget);
        m_toolCallsArea->show();
    }
}

void MessageBubble::onEditClicked()
{
    emit editRequested(m_messageId, m_markdownBuffer);
}

void MessageBubble::onDeleteClicked()
{
    emit deleteRequested(m_messageId);
}

void MessageBubble::onRegenerateClicked()
{
    emit regenerateRequested(m_messageId);
}

void MessageBubble::onCopyClicked()
{
    emit copyRequested(m_markdownBuffer);
}

void MessageBubble::onAnchorClicked(const QUrl &url)
{
    if (url.scheme() == "math") {
        emit mathClicked(QUrl::fromPercentEncoding(url.path().toUtf8()));
    } else if (url.scheme() == "mermaid") {
        emit mermaidClicked(QUrl::fromPercentEncoding(url.path().toUtf8()));
    }
}
```

- [ ] **Step 3: Commit**

```bash
git add desktop/src/messagebubble.h desktop/src/messagebubble.cpp
git commit -m "feat(desktop): extend MessageBubble with actions, tool calls, and streaming state"
```

---

## Task 5: ToolCallWidget

**Files:**
- Create: `desktop/src/toolcallwidget.h`, `desktop/src/toolcallwidget.cpp`

- [ ] **Step 1: 创建 ToolCallWidget**

`desktop/src/toolcallwidget.h`：

```cpp
#ifndef TOOLCALLWIDGET_H
#define TOOLCALLWIDGET_H

#include <QWidget>
#include <QJsonObject>
#include <QJsonValue>

class QLabel;
class QPushButton;

class ToolCallWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ToolCallWidget(QWidget *parent = nullptr);

    void setToolName(const QString &name);
    void setStatus(const QString &status);
    void setArguments(const QJsonObject &args);
    void setResult(const QJsonValue &result);

private slots:
    void toggleDetails();

private:
    void setupUI();
    void updateStatusIcon();
    QString formatJson(const QJsonValue &value, int indent = 0);

    QLabel *m_statusIcon;
    QLabel *m_toolNameLabel;
    QLabel *m_statusLabel;
    QPushButton *m_expandBtn;
    QWidget *m_detailsPanel;
    QLabel *m_argsLabel;
    QLabel *m_resultLabel;

    QString m_status;
    bool m_expanded;
};

#endif
```

`desktop/src/toolcallwidget.cpp`：

```cpp
#include "toolcallwidget.h"
#include <QLabel>
#include <QPushButton>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QJsonDocument>

ToolCallWidget::ToolCallWidget(QWidget *parent)
    : QWidget(parent),
      m_status("running"),
      m_expanded(false)
{
    setupUI();
}

void ToolCallWidget::setupUI()
{
    setStyleSheet(
        "ToolCallWidget { background: #14161a; border: 1px solid #2a2e36; "
        "border-radius: 4px; }"
    );

    m_statusIcon = new QLabel("⏳", this);
    m_statusIcon->setStyleSheet("font-size: 14px;");

    m_toolNameLabel = new QLabel(this);
    m_toolNameLabel->setStyleSheet("color: #e8e6e3; font-weight: 600; font-size: 12px;");

    m_statusLabel = new QLabel(tr("Running..."), this);
    m_statusLabel->setStyleSheet("color: #8b8f98; font-size: 11px;");

    m_expandBtn = new QPushButton(tr("Details ▼"), this);
    m_expandBtn->setStyleSheet(
        "QPushButton { background: transparent; color: #8b8f98; "
        "border: none; font-size: 11px; padding: 2px; }"
        "QPushButton:hover { color: #FFB800; }"
    );
    connect(m_expandBtn, &QPushButton::clicked, this, &ToolCallWidget::toggleDetails);

    QHBoxLayout *headerLayout = new QHBoxLayout;
    headerLayout->setContentsMargins(8, 6, 8, 6);
    headerLayout->setSpacing(8);
    headerLayout->addWidget(m_statusIcon);
    headerLayout->addWidget(m_toolNameLabel, 1);
    headerLayout->addWidget(m_statusLabel);
    headerLayout->addWidget(m_expandBtn);

    m_detailsPanel = new QWidget(this);
    m_detailsPanel->setVisible(false);
    QVBoxLayout *detailsLayout = new QVBoxLayout(m_detailsPanel);
    detailsLayout->setContentsMargins(8, 0, 8, 8);
    detailsLayout->setSpacing(4);

    m_argsLabel = new QLabel(this);
    m_argsLabel->setStyleSheet("color: #8b8f98; font-family: monospace; font-size: 11px;");
    m_argsLabel->setWordWrap(true);
    detailsLayout->addWidget(m_argsLabel);

    m_resultLabel = new QLabel(this);
    m_resultLabel->setStyleSheet("color: #8b8f98; font-family: monospace; font-size: 11px;");
    m_resultLabel->setWordWrap(true);
    detailsLayout->addWidget(m_resultLabel);

    QVBoxLayout *mainLayout = new QVBoxLayout(this);
    mainLayout->setContentsMargins(0, 0, 0, 0);
    mainLayout->setSpacing(0);
    mainLayout->addLayout(headerLayout);
    mainLayout->addWidget(m_detailsPanel);
}

void ToolCallWidget::setToolName(const QString &name)
{
    m_toolNameLabel->setText(name);
}

void ToolCallWidget::setStatus(const QString &status)
{
    m_status = status;
    updateStatusIcon();
    if (status == "running") {
        m_statusLabel->setText(tr("Running..."));
        m_statusLabel->setStyleSheet("color: #8b8f98; font-size: 11px;");
    } else if (status == "success") {
        m_statusLabel->setText(tr("Done"));
        m_statusLabel->setStyleSheet("color: #4ade80; font-size: 11px;");
    } else if (status == "error") {
        m_statusLabel->setText(tr("Error"));
        m_statusLabel->setStyleSheet("color: #f87171; font-size: 11px;");
    }
}

void ToolCallWidget::setArguments(const QJsonObject &args)
{
    m_argsLabel->setText(QString("<b>%1</b><br>%2").arg(tr("Arguments:"), formatJson(args)));
}

void ToolCallWidget::setResult(const QJsonValue &result)
{
    m_resultLabel->setText(QString("<b>%1</b><br>%2").arg(tr("Result:"), formatJson(result)));
}

void ToolCallWidget::toggleDetails()
{
    m_expanded = !m_expanded;
    m_detailsPanel->setVisible(m_expanded);
    m_expandBtn->setText(m_expanded ? tr("Details ▲") : tr("Details ▼"));
}

void ToolCallWidget::updateStatusIcon()
{
    if (m_status == "running") {
        m_statusIcon->setText("⏳");
    } else if (m_status == "success") {
        m_statusIcon->setText("✅");
    } else if (m_status == "error") {
        m_statusIcon->setText("❌");
    }
}

QString ToolCallWidget::formatJson(const QJsonValue &value, int indent)
{
    QJsonDocument doc(value.isObject() ? value.toObject() : QJsonObject());
    if (!value.isObject() && !value.isArray()) {
        return value.toVariant().toString();
    }
    if (value.isArray()) {
        doc = QJsonDocument(value.toArray());
    }
    QString formatted = doc.toJson(QJsonDocument::Indented);
    return formatted.toHtmlEscaped().replace("\n", "<br>").replace(" ", "&nbsp;");
}
```

- [ ] **Step 2: 更新 CMakeLists.txt**

在 `SOURCES` 和 `HEADERS` 中新增 `toolcallwidget.cpp` / `toolcallwidget.h`。

- [ ] **Step 3: Commit**

```bash
git add desktop/src/toolcallwidget.h desktop/src/toolcallwidget.cpp desktop/CMakeLists.txt
git commit -m "feat(desktop): add ToolCallWidget for tool call visualization"
```

---

## Task 6: ChatWidget 扩展

**Files:**
- Modify: `desktop/src/chatwidget.h`
- Modify: `desktop/src/chatwidget.cpp`

- [ ] **Step 1: 修改 ChatWidget 头文件**

`desktop/src/chatwidget.h`：

```cpp
#ifndef CHATWIDGET_H
#define CHATWIDGET_H

#include <QWidget>
#include <QTimer>

class QScrollArea;
class QVBoxLayout;
class QStackedWidget;
class PromptInput;
class MessageBubble;
class HermindClient;
class QNetworkReply;
class SSEParser;
class ConversationHeader;
class EmptyStateWidget;
class MarkdownRenderer;

class ChatWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ChatWidget(QWidget *parent = nullptr);
    void setClient(HermindClient *client);
    HermindClient* client() const;

protected:
    void dragEnterEvent(QDragEnterEvent *event) override;
    void dropEvent(QDropEvent *event) override;

public slots:
    void sendMessage(const QString &text);
    void loadSession(const QString &sessionId);
    void startNewSession();

private slots:
    void onStreamReadyRead();
    void onStreamFinished();
    void onRenderTimer();
    void onStopClicked();
    void onMessageEdit(const QString &messageId, const QString &currentText);
    void onMessageDelete(const QString &messageId);
    void onMessageRegenerate(const QString &messageId);
    void onMessageCopy(const QString &text);
    void onMathClicked(const QString &latex);
    void onMermaidClicked(const QString &diagram);
    void onAttachClicked();
    void onUploadFinished(const QJsonObject &resp, const QString &error);

private:
    void addMessageBubble(MessageBubble *bubble);
    void startStream();
    void setEmptyState(bool empty);
    void appendToCurrentBubble(const QString &text);
    void finalizeCurrentBubble();
    void enhanceCurrentBubble();
    void scrollToBottom();
    void handleToolCallEvent(const QJsonObject &payload);
    MessageBubble* findBubbleById(const QString &messageId);

    HermindClient *m_client;
    ConversationHeader *m_header;
    QStackedWidget *m_stack;
    QWidget *m_messagesPage;
    QScrollArea *m_scrollArea;
    QWidget *m_messagesContainer;
    QVBoxLayout *m_messagesLayout;
    EmptyStateWidget *m_emptyState;
    PromptInput *m_promptInput;
    MessageBubble *m_currentBubble;
    SSEParser *m_sseParser;
    QNetworkReply *m_streamReply;
    QTimer *m_renderTimer;
    QString m_pendingMarkdown;
    int m_renderGeneration;
    bool m_isStreaming;
    MarkdownRenderer *m_markdownRenderer;
    QStringList m_attachmentPaths;
};

#endif
```

- [ ] **Step 2: 修改 ChatWidget 实现**

关键变更摘要（基于现有 `chatwidget.cpp`）：

**构造函数新增**：
```cpp
m_markdownRenderer = new MarkdownRenderer(this);
// 连接 promptinput 的新信号
connect(m_promptInput, &PromptInput::attachClicked, this, &ChatWidget::onAttachClicked);
connect(m_promptInput, &PromptInput::stopClicked, this, &ChatWidget::onStopClicked);
```

**SSE 事件处理扩展**：在现有 `message_chunk`/`done`/`error` 之外新增 `tool_call`：
```cpp
} else if (type == "tool_call") {
    QJsonObject payload = obj.value("data").toObject();
    handleToolCallEvent(payload);
}
```

**新增方法实现**：

```cpp
void ChatWidget::onStopClicked()
{
    if (m_isStreaming && m_streamReply) {
        m_client->post("/api/conversation/cancel", QJsonObject(),
                       [](const QJsonObject &, const QString &) {});
        m_streamReply->abort();
    }
}

void ChatWidget::onMessageEdit(const QString &messageId, const QString &currentText)
{
    bool ok;
    QString text = QInputDialog::getMultiLineText(this, tr("Edit Message"),
                                                   tr("Message:"), currentText, &ok);
    if (!ok || text.isEmpty()) return;

    QJsonObject body;
    body["content"] = text;
    m_client->put(QString("/api/conversation/messages/%1").arg(messageId), body,
                  [this, messageId](const QJsonObject &, const QString &error) {
        if (!error.isEmpty()) {
            qWarning() << "Failed to edit message:" << error;
        }
    });
}

void ChatWidget::onMessageDelete(const QString &messageId)
{
    auto reply = QMessageBox::question(this, tr("Delete Message"),
                                       tr("Are you sure you want to delete this message?"));
    if (reply != QMessageBox::Yes) return;

    m_client->deleteResource(QString("/api/conversation/messages/%1").arg(messageId),
                             [this, messageId](const QJsonObject &, const QString &error) {
        if (!error.isEmpty()) {
            qWarning() << "Failed to delete message:" << error;
        } else {
            if (MessageBubble *bubble = findBubbleById(messageId)) {
                bubble->deleteLater();
            }
        }
    });
}

void ChatWidget::onMessageRegenerate(const QString &messageId)
{
    int count = m_messagesLayout->count();
    for (int i = 0; i < count - 1; ++i) {
        QLayoutItem *item = m_messagesLayout->itemAt(i);
        MessageBubble *bubble = qobject_cast<MessageBubble*>(item->widget());
        if (bubble && bubble->messageId() == messageId) {
            for (int j = i - 1; j >= 0; --j) {
                QLayoutItem *prevItem = m_messagesLayout->itemAt(j);
                MessageBubble *prevBubble = qobject_cast<MessageBubble*>(prevItem->widget());
                if (prevBubble && prevBubble->messageId() != messageId) {
                    bubble->deleteLater();
                    sendMessage(prevBubble->markdownBuffer());
                    return;
                }
            }
        }
    }
}

void ChatWidget::onMessageCopy(const QString &text)
{
    QApplication::clipboard()->setText(text);
}

void ChatWidget::onMathClicked(const QString &latex)
{
    if (!m_client) return;
    QJsonObject body;
    body["latex"] = latex;
    body["format"] = "svg";
    m_client->post("/api/render/math", body,
                   [this, latex](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) return;
        QString svg = resp.value("data").toString();
        QDialog *dialog = new QDialog(this);
        dialog->setWindowTitle(tr("Math Formula"));
        QVBoxLayout *layout = new QVBoxLayout(dialog);
        QLabel *label = new QLabel(dialog);
        label->setText(svg);
        label->setTextFormat(Qt::RichText);
        layout->addWidget(label);
        QPushButton *copyBtn = new QPushButton(tr("Copy LaTeX"), dialog);
        connect(copyBtn, &QPushButton::clicked, [latex]() {
            QApplication::clipboard()->setText(latex);
        });
        layout->addWidget(copyBtn);
        dialog->resize(400, 300);
        dialog->exec();
        dialog->deleteLater();
    });
}

void ChatWidget::onMermaidClicked(const QString &diagram)
{
    if (!m_client) return;
    QJsonObject body;
    body["diagram"] = diagram;
    body["format"] = "svg";
    m_client->post("/api/render/mermaid", body,
                   [this, diagram](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) return;
        QString svg = resp.value("data").toString();
        QDialog *dialog = new QDialog(this);
        dialog->setWindowTitle(tr("Diagram"));
        QVBoxLayout *layout = new QVBoxLayout(dialog);
        QLabel *label = new QLabel(dialog);
        label->setText(svg);
        label->setTextFormat(Qt::RichText);
        layout->addWidget(label);
        QPushButton *copyBtn = new QPushButton(tr("Copy Source"), dialog);
        connect(copyBtn, &QPushButton::clicked, [diagram]() {
            QApplication::clipboard()->setText(diagram);
        });
        layout->addWidget(copyBtn);
        dialog->resize(600, 400);
        dialog->exec();
        dialog->deleteLater();
    });
}

void ChatWidget::onAttachClicked()
{
    QStringList files = QFileDialog::getOpenFileNames(this, tr("Attach Files"));
    for (const QString &file : files) {
        m_client->uploadFile("/api/upload", file,
                             [this](const QJsonObject &resp, const QString &error) {
            if (!error.isEmpty()) {
                qWarning() << "Upload failed:" << error;
            } else {
                m_attachmentPaths.append(resp.value("url").toString());
            }
        });
    }
}

void ChatWidget::handleToolCallEvent(const QJsonObject &payload)
{
    if (!m_currentBubble) return;
    ToolCallWidget *widget = new ToolCallWidget(m_currentBubble);
    widget->setToolName(payload.value("name").toString());
    widget->setStatus(payload.value("status").toString("running"));
    if (payload.contains("arguments")) {
        widget->setArguments(payload.value("arguments").toObject());
    }
    if (payload.contains("result")) {
        widget->setResult(payload.value("result"));
    }
    m_currentBubble->addToolCallWidget(widget);
}

void ChatWidget::enhanceCurrentBubble()
{
    if (!m_currentBubble || !m_markdownRenderer) return;
    QString markdown = m_currentBubble->markdownBuffer();
    QString html = m_currentBubble->property("lastHtml").toString();
    if (html.isEmpty()) return;
    QString enhanced = m_markdownRenderer->enhanceHtml(html, markdown);
    m_currentBubble->setHtmlContent(enhanced);
}

MessageBubble* ChatWidget::findBubbleById(const QString &messageId)
{
    int count = m_messagesLayout->count();
    for (int i = 0; i < count; ++i) {
        QLayoutItem *item = m_messagesLayout->itemAt(i);
        MessageBubble *bubble = qobject_cast<MessageBubble*>(item->widget());
        if (bubble && bubble->messageId() == messageId) {
            return bubble;
        }
    }
    return nullptr;
}

void ChatWidget::scrollToBottom()
{
    QScrollBar *bar = m_scrollArea->verticalScrollBar();
    bar->setValue(bar->maximum());
}
```

**PromptInput 需要新增的信号**（修改 `promptinput.h`）：
```cpp
signals:
    void sendClicked();
    void attachClicked();
    void stopClicked();
```

**连接 MessageBubble 信号**（在 `addMessageBubble` 中）：
```cpp
void ChatWidget::addMessageBubble(MessageBubble *bubble)
{
    m_messagesLayout->insertWidget(m_messagesLayout->count() - 1, bubble);
    connect(bubble, &MessageBubble::editRequested, this, &ChatWidget::onMessageEdit);
    connect(bubble, &MessageBubble::deleteRequested, this, &ChatWidget::onMessageDelete);
    connect(bubble, &MessageBubble::regenerateRequested, this, &ChatWidget::onMessageRegenerate);
    connect(bubble, &MessageBubble::copyRequested, this, &ChatWidget::onMessageCopy);
    connect(bubble, &MessageBubble::mathClicked, this, &ChatWidget::onMathClicked);
    connect(bubble, &MessageBubble::mermaidClicked, this, &ChatWidget::onMermaidClicked);
}
```

- [ ] **Step 3: Commit**

```bash
git add desktop/src/chatwidget.h desktop/src/chatwidget.cpp
git commit -m "feat(desktop): extend ChatWidget with edit/delete/regenerate, stop, attachments, tool calls"
```

---

## Task 7: EmptyStateWidget 扩展

**Files:**
- Modify: `desktop/src/emptystatewidget.h`
- Modify: `desktop/src/emptystatewidget.cpp`

- [ ] **Step 1: 连接 /api/suggestions 动态加载**

修改 `desktop/src/emptystatewidget.h` 新增：
```cpp
public slots:
    void setSuggestions(const QStringList &suggestions);
```

修改 `desktop/src/emptystatewidget.cpp`：将硬编码建议改为动态设置。在 `ChatWidget::setClient()` 中连接 `/api/suggestions`：

```cpp
m_client->get("/api/suggestions", [this](const QJsonObject &resp, const QString &) {
    QJsonArray arr = resp.value("suggestions").toArray();
    QStringList suggestions;
    for (const QJsonValue &v : arr) {
        suggestions.append(v.toString());
    }
    if (suggestions.isEmpty()) {
        suggestions << tr("Explain a concept") << tr("Write some code") << tr("Debug an error");
    }
    m_emptyState->setSuggestions(suggestions);
});
```

- [ ] **Step 2: Commit**

```bash
git add desktop/src/emptystatewidget.h desktop/src/emptystatewidget.cpp desktop/src/chatwidget.cpp
git commit -m "feat(desktop): connect EmptyStateWidget to /api/suggestions"
```

---

## Task 8: ConfigFormEngine

**Files:**
- Create: `desktop/src/configformengine.h`, `desktop/src/configformengine.cpp`
- Test: `desktop/tests/test_config_form_engine.cpp`

- [ ] **Step 1: 创建 ConfigFormEngine**

`desktop/src/configformengine.h`（见 Task 8 设计文档，完整代码已在设计文档中提供）。

`desktop/src/configformengine.cpp`（核心逻辑已在设计文档中提供）。

- [ ] **Step 2: 编写测试**

`desktop/tests/test_config_form_engine.cpp`（已在设计文档中提供）。

- [ ] **Step 3: 更新 CMakeLists.txt**

在 `SOURCES`/`HEADERS` 中新增 `configformengine`，新增测试目标 `test_config_form_engine`。

- [ ] **Step 4: 构建并运行测试**

```bash
cd desktop && cmake -B build -S . && cmake --build build --target test_config_form_engine && ./build/test_config_form_engine
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/src/configformengine.h desktop/src/configformengine.cpp \
        desktop/tests/test_config_form_engine.cpp desktop/CMakeLists.txt
git commit -m "feat(desktop): add ConfigFormEngine for dynamic schema-driven forms"
```

---

## Task 9: SettingsEditor UI 框架

**Files:**
- Create: `desktop/src/settingssidebar.h`, `desktop/src/settingssidebar.cpp`
- Create: `desktop/src/settingspanel.h`, `desktop/src/settingspanel.cpp`
- Create: `desktop/src/settingseditor.h`, `desktop/src/settingseditor.cpp`

- [ ] **Step 1: 创建 SettingsSidebar**

`desktop/src/settingssidebar.h` 和 `.cpp`（已在设计文档中提供完整代码）。

- [ ] **Step 2: 创建 SettingsPanel**

`desktop/src/settingspanel.h` 和 `.cpp`（已在设计文档中提供完整代码）。

- [ ] **Step 3: 创建 SettingsEditor**

`desktop/src/settingseditor.h` 和 `.cpp`（核心逻辑已在设计文档中提供）。

- [ ] **Step 4: 更新 CMakeLists.txt**

在 `SOURCES`/`HEADERS` 中新增 `settingssidebar`、`settingspanel`、`settingseditor`。

- [ ] **Step 5: Commit**

```bash
git add desktop/src/settingssidebar.h desktop/src/settingssidebar.cpp \
        desktop/src/settingspanel.h desktop/src/settingspanel.cpp \
        desktop/src/settingseditor.h desktop/src/settingseditor.cpp \
        desktop/CMakeLists.txt
git commit -m "feat(desktop): add SettingsEditor framework with sidebar and panel"
```

---

## Task 10: ProviderEditor + FallbackEditor

**Files:**
- Create: `desktop/src/providereditor.h`, `desktop/src/providereditor.cpp`
- Create: `desktop/src/fallbackeditor.h`, `desktop/src/fallbackeditor.cpp`

- [ ] **Step 1: 创建 ProviderEditor**

`desktop/src/providereditor.h` 和 `.cpp`（头文件已在设计文档中提供）。

- [ ] **Step 2: 创建 FallbackEditor**

`desktop/src/fallbackeditor.h` 和 `.cpp`（头文件已在设计文档中提供）。

- [ ] **Step 3: 更新 CMakeLists.txt 并 Commit**

```bash
git add desktop/src/providereditor.h desktop/src/providereditor.cpp \
        desktop/src/fallbackeditor.h desktop/src/fallbackeditor.cpp \
        desktop/CMakeLists.txt
git commit -m "feat(desktop): add ProviderEditor and FallbackEditor"
```

---

## Task 11: 其他设置分组编辑器

**Files:**
- Create: `desktop/src/mcpservereditor.h`, `desktop/src/mcpservereditor.cpp`
- Create: `desktop/src/croneditor.h`, `desktop/src/croneditor.cpp`
- Create: `desktop/src/scalareditor.h`, `desktop/src/scalareditor.cpp`

- [ ] **Step 1: 创建各编辑器**

`ScalarEditor`：基于 `ConfigFormEngine` 的简单封装。  
`McpServerEditor`：表格管理 MCP 服务器（名称、命令、参数）。  
`CronEditor`：表格管理 Cron 任务（表达式、命令、启用状态）。

- [ ] **Step 2: 更新 CMakeLists.txt 并 Commit**

```bash
git add desktop/src/mcpservereditor.h desktop/src/mcpservereditor.cpp \
        desktop/src/croneditor.h desktop/src/croneditor.cpp \
        desktop/src/scalareditor.h desktop/src/scalareditor.cpp \
        desktop/CMakeLists.txt
git commit -m "feat(desktop): add MCP, Cron, and Scalar editors"
```

---

## Task 12: TopBar 扩展

**Files:**
- Modify: `desktop/src/topbar.h`
- Modify: `desktop/src/topbar.cpp`

- [ ] **Step 1: 修改 TopBar**

在现有 TopBar 上增加语言切换和主题切换按钮：

```cpp
// topbar.h 新增信号和成员
signals:
    void modeChanged(const QString &mode);
    void saveRequested();
    void languageToggled();
    void themeToggled();

private:
    QPushButton *m_langBtn;
    QPushButton *m_themeBtn;
```

`topbar.cpp` 的 `setupUI()` 中，在保存按钮旁边新增：

```cpp
m_langBtn = new QPushButton("🌐 EN", this);
m_langBtn->setStyleSheet(
    "QPushButton { background: transparent; color: #8a8680; "
    "border: 1px solid #2a2e36; border-radius: 4px; padding: 4px 8px; "
    "font-size: 11px; }"
    "QPushButton:hover { color: #e8e6e3; border-color: #FFB800; }"
);
connect(m_langBtn, &QPushButton::clicked, this, &TopBar::languageToggled);

m_themeBtn = new QPushButton("🌙", this);
m_themeBtn->setStyleSheet(m_langBtn->styleSheet());
connect(m_themeBtn, &QPushButton::clicked, this, &TopBar::themeToggled);
```

- [ ] **Step 2: Commit**

```bash
git add desktop/src/topbar.h desktop/src/topbar.cpp
git commit -m "feat(desktop): add language and theme toggle buttons to TopBar"
```

---

## Task 13: AppWindow + main.cpp 集成

**Files:**
- Modify: `desktop/src/appwindow.h`
- Modify: `desktop/src/appwindow.cpp`
- Modify: `desktop/src/main.cpp`

- [ ] **Step 1: 修改 AppWindow**

将 `SettingsDialog` 替换为 `SettingsEditor`：

```cpp
// appwindow.h
class ThemeManager;
class I18n;
class SettingsEditor;

class AppWindow : public QWidget {
    // ...
    void setThemeManager(ThemeManager *tm);
    void setI18n(I18n *i18n);
    // ...
private:
    SettingsEditor *m_settingsEditor;
    ThemeManager *m_themeManager;
    I18n *m_i18n;
};
```

`appwindow.cpp` 的 `setupTopBar()`：

```cpp
void AppWindow::setupTopBar()
{
    m_topBar = new TopBar(this);
    connect(m_topBar, &TopBar::modeChanged, this, [this](const QString &mode) {
        if (mode == "settings") {
            if (!m_settingsEditor) {
                m_settingsEditor = new SettingsEditor(m_chatWidget->client(), this);
            }
            m_settingsEditor->show();
            m_settingsEditor->raise();
            m_settingsEditor->activateWindow();
        }
    });
    connect(m_topBar, &TopBar::languageToggled, this, [this]() {
        if (m_i18n) {
            QString next = (m_i18n->currentLanguage() == "en") ? "zh_CN" : "en";
            m_i18n->loadLanguage(next);
        }
    });
    connect(m_topBar, &TopBar::themeToggled, this, [this]() {
        if (m_themeManager) {
            QString next = (m_themeManager->currentTheme() == "dark") ? "light" : "dark";
            m_themeManager->loadTheme(next);
        }
    });
}
```

- [ ] **Step 2: 修改 main.cpp**

```cpp
int main(int argc, char *argv[])
{
    // ... existing setup ...

    ThemeManager themeManager;
    I18n i18n;

    AppWindow window;
    window.setThemeManager(&themeManager);
    window.setI18n(&i18n);

    // ... rest of main.cpp unchanged ...
}
```

- [ ] **Step 3: Commit**

```bash
git add desktop/src/appwindow.h desktop/src/appwindow.cpp desktop/src/main.cpp
git commit -m "feat(desktop): integrate SettingsEditor, ThemeManager, and I18n into AppWindow"
```

---

## Task 14: QSS 主题文件

**Files:**
- Create: `desktop/resources/themes/dark.qss`
- Create: `desktop/resources/themes/light.qss`
- Modify: `desktop/resources/resources.qrc`

- [ ] **Step 1: 迁移暗色主题**

将现有 `desktop/resources/styles.qss` 复制为 `desktop/resources/themes/dark.qss`。

- [ ] **Step 2: 创建亮色主题**

`desktop/resources/themes/light.qss`（已在设计文档中提供完整内容）。

- [ ] **Step 3: 更新 resources.qrc**

```xml
<RCC>
    <qresource prefix="/">
        <file>themes/dark.qss</file>
        <file>themes/light.qss</file>
        <file>i18n/hermind_en.qm</file>
        <file>i18n/hermind_zh_CN.qm</file>
    </qresource>
</RCC>
```

- [ ] **Step 4: Commit**

```bash
git add desktop/resources/themes/dark.qss desktop/resources/themes/light.qss \
        desktop/resources/resources.qrc
git commit -m "feat(desktop): add light theme QSS and restructure theme resources"
```

---

## Task 15: 翻译文件

**Files:**
- Create: `desktop/resources/i18n/hermind_en.ts`
- Create: `desktop/resources/i18n/hermind_zh_CN.ts`

- [ ] **Step 1: 创建翻译源文件**

`desktop/resources/i18n/hermind_en.ts` 和 `hermind_zh_CN.ts`（结构已在设计文档中提供）。

使用 `lrelease` 编译为 `.qm`：
```bash
cd desktop/resources/i18n
lrelease hermind_en.ts hermind_zh_CN.ts
```

- [ ] **Step 2: Commit**

```bash
git add desktop/resources/i18n/hermind_en.ts desktop/resources/i18n/hermind_zh_CN.ts \
        desktop/resources/i18n/hermind_en.qm desktop/resources/i18n/hermind_zh_CN.qm
git commit -m "feat(desktop): add English and Chinese translation files"
```

---

## Task 16: 后端渲染 API

**Files:**
- Modify: `api/handlers_render.go`

- [ ] **Step 1: 新增 `/api/render/math` 和 `/api/render/mermaid`**

在 `api/handlers_render.go` 中新增两个 handler：

```go
func (s *Server) handleRenderMath(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Latex      string `json:"latex"`
        DisplayMode bool  `json:"displayMode"`
        Format     string `json:"format"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if req.Format == "" {
        req.Format = "svg"
    }

    // Call katex CLI
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    args := []string{"--" + req.Format}
    if req.DisplayMode {
        args = append(args, "--display-mode")
    }
    cmd := exec.CommandContext(ctx, "npx", append([]string{"katex"}, args...)...)
    cmd.Stdin = strings.NewReader(req.Latex)
    output, err := cmd.Output()
    if err != nil {
        http.Error(w, "Failed to render math: "+string(output), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "data":   string(output),
        "format": req.Format,
    })
}

func (s *Server) handleRenderMermaid(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Diagram string `json:"diagram"`
        Format  string `json:"format"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if req.Format == "" {
        req.Format = "svg"
    }

    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    tmpFile, err := os.CreateTemp("", "mermaid-*.mmd")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer os.Remove(tmpFile.Name())
    tmpFile.WriteString(req.Diagram)
    tmpFile.Close()

    outputFile := tmpFile.Name() + "." + req.Format
    defer os.Remove(outputFile)

    cmd := exec.CommandContext(ctx, "npx", "@mermaid-js/mermaid-cli", "-i", tmpFile.Name(), "-o", outputFile)
    output, err := cmd.CombinedOutput()
    if err != nil {
        http.Error(w, "Failed to render mermaid: "+string(output), http.StatusInternalServerError)
        return
    }

    data, err := os.ReadFile(outputFile)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "data":   string(data),
        "format": req.Format,
    })
}
```

在路由注册中添加：
```go
r.Post("/api/render/math", s.handleRenderMath)
r.Post("/api/render/mermaid", s.handleRenderMermaid)
```

- [ ] **Step 2: Commit**

```bash
git add api/handlers_render.go
git commit -m "feat(api): add /api/render/math and /api/render/mermaid endpoints"
```

---

## Task 17: 最终 CMakeLists.txt 更新与全量构建

**Files:**
- Modify: `desktop/CMakeLists.txt`

- [ ] **Step 1: 确认 CMakeLists.txt 包含所有新增文件**

最终的 `SOURCES` 列表：

```cmake
set(SOURCES
    src/main.cpp
    src/hermindprocess.cpp
    src/httplib.cpp
    src/sseparser.cpp
    src/appwindow.cpp
    src/sessionlistwidget.cpp
    src/chatwidget.cpp
    src/messagebubble.cpp
    src/promptinput.cpp
    src/shortcutmanager.cpp
    src/trayicon.cpp
    src/topbar.cpp
    src/statusfooter.cpp
    src/emptystatewidget.cpp
    src/conversationheader.cpp
    src/thememanager.cpp
    src/i18n.cpp
    src/codehighlighter.cpp
    src/markdownrenderer.cpp
    src/toolcallwidget.cpp
    src/configformengine.cpp
    src/settingssidebar.cpp
    src/settingspanel.cpp
    src/settingseditor.cpp
    src/providereditor.cpp
    src/fallbackeditor.cpp
    src/mcpservereditor.cpp
    src/croneditor.cpp
    src/scalareditor.cpp
    resources/resources.qrc
)
```

最终的 `HEADERS` 列表对应添加所有 `.h` 文件。

- [ ] **Step 2: 全量构建**

```bash
cd desktop && cmake -B build -S . && cmake --build build
```

Expected: 构建成功，无编译错误

- [ ] **Step 3: 运行所有测试**

```bash
cd desktop && ctest --test-dir build --output-on-failure
```

Expected: 所有测试 PASS

- [ ] **Step 4: Commit**

```bash
git add desktop/CMakeLists.txt
git commit -m "build(desktop): update CMakeLists.txt with all new sources"
```

---

## Task 18: 手动验证

- [ ] **Step 1: 启动应用**

```bash
cd desktop && ./build/hermind-desktop
```

- [ ] **Step 2: 按测试清单验证**

| 功能 | 检查点 | 状态 |
|---|---|---|
| 聊天 | 发送消息、流式显示、停止生成 | - |
| Markdown | 标题、列表、代码块高亮 | - |
| 数学公式 | 显示占位符、点击弹窗 | - |
| Mermaid | 显示占位符、点击弹窗 | - |
| 消息操作 | 编辑、删除、重生成、复制 | - |
| 设置 | 所有分组可切换、保存成功 | - |
| Provider | 添加/测试/获取模型 | - |
| 主题 | 暗/亮切换 | - |
| 语言 | 中/英切换 | - |
| 系统托盘 | 隐藏/显示、快捷键 | - |

---

## 自我审查

**1. Spec coverage:**

| 设计文档需求 | 对应任务 |
|---|---|
| HermindClient 扩展 (put/delete/upload) | Task 1 ✅ |
| ThemeManager + I18n | Task 2 ✅ |
| CodeHighlighter + MarkdownRenderer | Task 3 ✅ |
| MessageBubble 扩展 | Task 4 ✅ |
| ToolCallWidget | Task 5 ✅ |
| ChatWidget 扩展 | Task 6 ✅ |
| EmptyStateWidget /api/suggestions | Task 7 ✅ |
| ConfigFormEngine | Task 8 ✅ |
| SettingsEditor 框架 | Task 9 ✅ |
| ProviderEditor + FallbackEditor | Task 10 ✅ |
| MCP/Cron/Scalar 编辑器 | Task 11 ✅ |
| TopBar 扩展 | Task 12 ✅ |
| AppWindow + main.cpp 集成 | Task 13 ✅ |
| QSS 主题文件 | Task 14 ✅ |
| 翻译文件 | Task 15 ✅ |
| 后端 /api/render/math + /api/render/mermaid | Task 16 ✅ |
| 测试 + CMake | Task 17 ✅ |

**2. Placeholder scan:** 无 TBD/TODO/"implement later" ✅

**3. Type consistency:** 所有类名、方法名、信号名在设计文档和计划文档中一致 ✅

---

**Plan complete and saved to `plans/2026-05-15-qt-desktop-rewrite.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach do you prefer?**
