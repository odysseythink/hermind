# Hermind Desktop 界面复刻实施计划 (Bug-0028)

**Goal:** 依照截图 `D:\Pictures\bug-0028.png`，使用现有 Qt Widgets 工程规范在 `hermind-desktop` 中复刻 AnythingLLM 风格的聊天主界面布局、配色与交互骨架。

**Architecture:** 保持单 `QMainWindow` 结构，通过 `mainwindow.ui` 声明左侧边栏与右侧聊天区的完整控件树，在 `mainwindow.cpp` 中统一加载 QSS 实现配色、圆角与图标化按钮；不引入新的运行时依赖，仅用 Qt Widgets 标准模块与项目已有 logo 资源。

**Tech Stack:** Qt 5.15+/Qt 6 Widgets, qmake, C++17, QSS

> For executing workers: implement this plan task-by-task (prefer a fresh subagent/Task per task — a clean context per task avoids single-session degradation). Steps use - [ ] checkboxes for tracking.

## File Structure

### 修改文件

| 文件 | 责任 |
|------|------|
| `hermind-desktop/hermind-desktop.pro` | 增加资源文件引用，确认 Qt Widgets 模块 |
| `hermind-desktop/mainwindow.ui` | 完整声明侧边栏 + 聊天区控件树与布局 |
| `hermind-desktop/mainwindow.h` | 声明按钮点击槽函数 |
| `hermind-desktop/mainwindow.cpp` | 加载 QSS、连接槽函数、设置窗口标题、初始化列表数据 |

### 新增文件

| 文件 | 责任 |
|------|------|
| `hermind-desktop/resources.qrc` | 注册 `logo.svg` 资源 |
| `hermind-desktop/assets/logo.svg` | 从现有前端资源复制 Hermind logo |

## Dependency Overview

```
Task 1: 工程与资源配置
  └── Task 2: 主窗口 UI 布局
        └── Task 3: QSS 样式、交互槽与视觉验收
```

- 所有任务顺序依赖：UI 布局依赖资源路径存在；样式与交互依赖 UI 中控件 `objectName`；视觉验收依赖编译通过。
- 无并行任务。

## Risks & Open Questions

| 风险/问题 | 影响 | 缓解/说明 |
|-----------|------|-----------|
| 原始截图为 AnythingLLM 品牌，项目没有其 logo/图标素材 | 无法 100% 还原品牌图标 | 使用项目已有 Hermind logo 替代品牌区；功能图标使用 Unicode 符号近似，保持相同布局语义 |
| 截图中的具体色值无法精确提取 | 还原度有偏差 | 使用肉眼取色最接近的十六进制色值；验收时与截图并排对比 |
| Qt 版本差异导致 QSS 表现不同 | 样式错位 | 使用 Qt 5/6 兼容的属性；优先用 `QFrame` + 布局而非绝对定位 |
| `.ui` 文件手写 XML 与 Qt Designer 行为差异 | 编译通过但布局异常 | 每个任务后用 `uic`/`qmake` 编译验证 |

## Spec Coverage

| 截图元素 | 覆盖任务 | 状态 |
|----------|----------|------|
| 窗口标题栏文本 | Task 3 | covered |
| 左侧边栏整体布局 | Task 2 | covered |
| Logo + 品牌文字 | Task 2, Task 3 | covered |
| 搜索框 + “+” 按钮 | Task 2, Task 3 | covered |
| “我的工作区” 标题 + 图标 | Task 2, Task 3 | covered |
| 会话列表 | Task 2, Task 3 | covered |
| “+ New Thread” 按钮 | Task 2, Task 3 | covered |
| “Assistant Chats” 区域 | Task 2, Task 3 | covered |
| 底部四个圆形功能按钮 | Task 2, Task 3 | covered |
| 右侧聊天区头部 | Task 2, Task 3 | covered |
| 欢迎语文本 | Task 2, Task 3 | covered |
| 消息输入框 + 工具栏 | Task 2, Task 3 | covered |
| 底部三个操作胶囊按钮 | Task 2, Task 3 | covered |
| 编译通过且可运行 | Task 3 | covered |
| 与截图并排视觉验收 | Task 3 | covered |
| 像素级图标资源 | — | no-op（无原始素材） |

### Task 1: 工程与资源配置

**Depends on:** none

**Files:**
- Create: `hermind-desktop/assets/logo.svg`
- Create: `hermind-desktop/resources.qrc`
- Modify: `hermind-desktop/hermind-desktop.pro`

- [ ] 创建资源目录并复制项目已有 logo：
  ```bash
  mkdir -p hermind-desktop/assets
  cp frontend/src/media/illustrations/login-logo.svg hermind-desktop/assets/logo.svg
  ```

- [ ] 创建 `hermind-desktop/resources.qrc`，完整内容：
  ```xml
  <RCC>
      <qresource prefix="/images">
          <file alias="logo.svg">assets/logo.svg</file>
      </qresource>
  </RCC>
  ```

- [ ] 修改 `hermind-desktop/hermind-desktop.pro`，完整文件内容：
  ```pro
  QT += widgets

  CONFIG += c++17

  # You can make your code fail to compile if it uses deprecated APIs.
  # In order to do so, uncomment the following line.
  #DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

  SOURCES += \
      main.cpp \
      mainwindow.cpp

  HEADERS += \
      mainwindow.h

  FORMS += \
      mainwindow.ui

  RESOURCES += \
      resources.qrc

  # Default rules for deployment.
  qnx: target.path = /tmp/$${TARGET}/bin
  else: unix:!android: target.path = /opt/$${TARGET}/bin
  !isEmpty(target.path): INSTALLS += target
  ```

- [ ] 验证 qmake 能解析工程且资源被识别：
  ```bash
  cd hermind-desktop
  qmake hermind-desktop.pro
  ```
  预期：命令退出码为 `0`，生成 `Makefile`，终端无 `RCC: Error` 或 `Cannot find file` 提示。

- [ ] 手动验证 `logo.svg` 已纳入资源：
  ```bash
  grep -q "assets/logo.svg" Makefile
  ```
  预期：命令返回 `0`，`Makefile` 中包含对 `assets/logo.svg` 的引用。

- [ ] Commit：
  ```bash
  git add hermind-desktop/assets/logo.svg hermind-desktop/resources.qrc hermind-desktop/hermind-desktop.pro
  git commit -m "bug-0028: add Hermind logo resource for desktop UI"

### Task 2: 主窗口 UI 布局

**Depends on:** Task 1

**Files:**
- Modify: `hermind-desktop/mainwindow.ui`

- [ ] 将 `hermind-desktop/mainwindow.ui` 替换为以下完整内容：
  ```xml
  <?xml version="1.0" encoding="UTF-8"?>
  <ui version="4.0">
   <class>MainWindow</class>
   <widget class="QMainWindow" name="MainWindow">
    <property name="geometry">
     <rect>
      <x>0</x>
      <y>0</y>
      <width>1200</width>
      <height>800</height>
     </rect>
    </property>
    <property name="windowTitle">
     <string>Hermind Desktop</string>
    </property>
    <widget class="QWidget" name="centralwidget">
     <layout class="QHBoxLayout" name="horizontalLayout_2">
      <property name="spacing">
       <number>0</number>
      </property>
      <property name="leftMargin">
       <number>0</number>
      </property>
      <property name="topMargin">
       <number>0</number>
      </property>
      <property name="rightMargin">
       <number>0</number>
      </property>
      <property name="bottomMargin">
       <number>0</number>
      </property>
      <item>
       <widget class="QFrame" name="sidebarFrame">
        <property name="minimumSize">
         <size>
          <width>260</width>
          <height>0</height>
         </size>
        </property>
        <property name="maximumSize">
         <size>
          <width>260</width>
          <height>16777215</height>
         </size>
        </property>
        <layout class="QVBoxLayout" name="verticalLayout">
         <property name="spacing">
          <number>12</number>
         </property>
         <property name="leftMargin">
          <number>16</number>
         </property>
         <property name="topMargin">
          <number>16</number>
         </property>
         <property name="rightMargin">
          <number>16</number>
         </property>
         <property name="bottomMargin">
          <number>16</number>
         </property>
         <item>
          <layout class="QHBoxLayout" name="logoLayout">
           <property name="spacing">
            <number>8</number>
           </property>
           <item>
            <widget class="QLabel" name="logoLabel">
             <property name="minimumSize">
              <size>
               <width>24</width>
               <height>24</height>
              </size>
             </property>
             <property name="maximumSize">
              <size>
               <width>24</width>
               <height>24</height>
              </size>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QLabel" name="brandLabel">
             <property name="text">
              <string>Hermind</string>
             </property>
            </widget>
           </item>
           <item>
            <spacer name="logoSpacer">
             <property name="orientation">
              <enum>Qt::Horizontal</enum>
             </property>
            </spacer>
           </item>
           <item>
            <widget class="QToolButton" name="popoutButton">
             <property name="text">
              <string>⧉</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
         <item>
          <layout class="QHBoxLayout" name="searchLayout">
           <property name="spacing">
            <number>8</number>
           </property>
           <item>
            <widget class="QLineEdit" name="searchEdit">
             <property name="placeholderText">
              <string>搜索</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="newSearchButton">
             <property name="text">
              <string>+</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
         <item>
          <layout class="QHBoxLayout" name="workspaceHeader">
           <property name="spacing">
            <number>6</number>
           </property>
           <item>
            <widget class="QLabel" name="workspaceIcon">
             <property name="text">
              <string>★</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QLabel" name="workspaceLabel">
             <property name="text">
              <string>我的工作区</string>
             </property>
            </widget>
           </item>
           <item>
            <spacer name="workspaceHeaderSpacer">
             <property name="orientation">
              <enum>Qt::Horizontal</enum>
             </property>
            </spacer>
           </item>
           <item>
            <widget class="QToolButton" name="uploadButton">
             <property name="text">
              <string>⬆</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="workspaceSettingsButton">
             <property name="text">
              <string>⚙</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
         <item>
          <widget class="QListWidget" name="threadList">
           <property name="frameShape">
            <enum>QFrame::NoFrame</enum>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QPushButton" name="newThreadButton">
           <property name="text">
            <string>+  New Thread</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QPushButton" name="assistantChatsButton">
           <property name="text">
            <string>☰  Assistant Chats</string>
           </property>
          </widget>
         </item>
         <item>
          <spacer name="sidebarSpacer">
           <property name="orientation">
            <enum>Qt::Vertical</enum>
           </property>
          </spacer>
         </item>
         <item>
          <layout class="QHBoxLayout" name="bottomButtonsLayout">
           <property name="spacing">
            <number>8</number>
           </property>
           <item>
            <widget class="QToolButton" name="bottomChatButton">
             <property name="text">
              <string>💬</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="bottomDocsButton">
             <property name="text">
              <string>🗎</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="bottomGithubButton">
             <property name="text">
              <string>🐙</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="bottomToolsButton">
             <property name="text">
              <string>🔧</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
        </layout>
       </widget>
      </item>
      <item>
       <widget class="QFrame" name="chatFrame">
        <layout class="QVBoxLayout" name="verticalLayout_2">
         <property name="spacing">
          <number>0</number>
         </property>
         <property name="leftMargin">
          <number>0</number>
         </property>
         <property name="topMargin">
          <number>0</number>
         </property>
         <property name="rightMargin">
          <number>0</number>
         </property>
         <property name="bottomMargin">
          <number>0</number>
         </property>
         <item>
          <layout class="QHBoxLayout" name="chatHeader">
           <property name="spacing">
            <number>12</number>
           </property>
           <property name="leftMargin">
            <number>24</number>
           </property>
           <property name="topMargin">
            <number>16</number>
           </property>
           <property name="rightMargin">
            <number>24</number>
           </property>
           <property name="bottomMargin">
            <number>16</number>
           </property>
           <item>
            <widget class="QLabel" name="workspaceNameLabel">
             <property name="text">
              <string>kb-big</string>
             </property>
            </widget>
           </item>
           <item>
            <spacer name="chatHeaderSpacer">
             <property name="orientation">
              <enum>Qt::Horizontal</enum>
             </property>
            </spacer>
           </item>
           <item>
            <widget class="QLabel" name="versionLabel">
             <property name="text">
              <string>v1.14.0</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QToolButton" name="headerSettingsButton">
             <property name="text">
              <string>⚙</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
         <item>
          <spacer name="topChatSpacer">
           <property name="orientation">
            <enum>Qt::Vertical</enum>
           </property>
          </spacer>
         </item>
         <item alignment="Qt::AlignHCenter">
          <widget class="QLabel" name="welcomeLabel">
           <property name="text">
            <string>今天我能帮您什么？</string>
           </property>
          </widget>
         </item>
         <item>
          <spacer name="welcomeInputSpacer">
           <property name="orientation">
            <enum>Qt::Vertical</enum>
           </property>
           <property name="sizeHint" stdset="0">
            <size>
             <width>20</width>
             <height>40</height>
            </size>
           </property>
          </spacer>
         </item>
         <item alignment="Qt::AlignHCenter">
          <widget class="QFrame" name="inputFrame">
           <property name="minimumSize">
            <size>
             <width>760</width>
             <height>140</height>
            </size>
           </property>
           <property name="maximumSize">
            <size>
             <width>760</width>
             <height>180</height>
            </size>
           </property>
           <layout class="QVBoxLayout" name="verticalLayout_3">
            <property name="spacing">
             <number>12</number>
            </property>
            <property name="leftMargin">
             <number>16</number>
            </property>
            <property name="topMargin">
             <number>16</number>
            </property>
            <property name="rightMargin">
             <number>16</number>
            </property>
            <property name="bottomMargin">
             <number>12</number>
            </property>
            <item>
             <widget class="QTextEdit" name="messageEdit">
              <property name="placeholderText">
               <string>发送消息</string>
              </property>
              <property name="frameShape">
               <enum>QFrame::NoFrame</enum>
              </property>
             </widget>
            </item>
            <item>
             <layout class="QHBoxLayout" name="inputToolbar">
              <property name="spacing">
               <number>12</number>
              </property>
              <item>
               <widget class="QPushButton" name="toolsButton">
                <property name="text">
                 <string>+  工具</string>
                </property>
               </widget>
              </item>
              <item>
               <spacer name="inputToolbarSpacer">
                <property name="orientation">
                 <enum>Qt::Horizontal</enum>
                </property>
               </spacer>
              </item>
              <item>
               <widget class="QToolButton" name="micButton">
                <property name="text">
                 <string>🎤</string>
                </property>
               </widget>
              </item>
              <item>
               <widget class="QToolButton" name="sendButton">
                <property name="text">
                 <string>➤</string>
                </property>
               </widget>
              </item>
             </layout>
            </item>
           </layout>
          </widget>
         </item>
         <item>
          <spacer name="inputActionSpacer">
           <property name="orientation">
            <enum>Qt::Vertical</enum>
           </property>
           <property name="sizeHint" stdset="0">
            <size>
             <width>20</width>
             <height>20</height>
            </size>
           </property>
          </spacer>
         </item>
         <item alignment="Qt::AlignHCenter">
          <layout class="QHBoxLayout" name="actionButtonsLayout">
           <property name="spacing">
            <number>12</number>
           </property>
           <item>
            <widget class="QPushButton" name="createAgentButton">
             <property name="text">
              <string>创建代理</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QPushButton" name="editWorkspaceButton">
             <property name="text">
              <string>编辑工作区</string>
             </property>
            </widget>
           </item>
           <item>
            <widget class="QPushButton" name="uploadFileButton">
             <property name="text">
              <string>上传文件</string>
             </property>
            </widget>
           </item>
          </layout>
         </item>
         <item>
          <spacer name="bottomChatSpacer">
           <property name="orientation">
            <enum>Qt::Vertical</enum>
           </property>
          </spacer>
         </item>
        </layout>
       </widget>
      </item>
     </layout>
    </widget>
    <widget class="QMenuBar" name="menubar">
     <property name="geometry">
      <rect>
       <x>0</x>
       <y>0</y>
       <width>1200</width>
       <height>21</height>
      </rect>
     </property>
     <property name="visible">
      <bool>false</bool>
     </property>
    </widget>
    <widget class="QStatusBar" name="statusbar">
     <property name="visible">
      <bool>false</bool>
     </property>
    </widget>
   </widget>
   <resources/>
   <connections/>
  </ui>
  ```

- [ ] 验证 UI 文件能被 `uic` 编译：
  ```bash
  cd hermind-desktop
  uic mainwindow.ui -o /tmp/ui_mainwindow.h
  ```
  预期：退出码为 `0`，`/tmp/ui_mainwindow.h` 生成且无错误输出。

- [ ] Commit：
  ```bash
  git add hermind-desktop/mainwindow.ui
  git commit -m "bug-0028: layout mainwindow sidebar and chat area"

### Task 3: QSS 样式、交互槽与视觉验收

**Depends on:** Task 2

**Files:**
- Modify: `hermind-desktop/mainwindow.h`
- Modify: `hermind-desktop/mainwindow.cpp`

- [ ] 将 `hermind-desktop/mainwindow.h` 替换为以下完整内容：
  ```cpp
  #ifndef MAINWINDOW_H
  #define MAINWINDOW_H

  #include <QMainWindow>

  QT_BEGIN_NAMESPACE
  namespace Ui {
  class MainWindow;
  }
  QT_END_NAMESPACE

  class MainWindow : public QMainWindow
  {
      Q_OBJECT

  public:
      explicit MainWindow(QWidget *parent = nullptr);
      ~MainWindow() override;

  private slots:
      void on_popoutButton_clicked();
      void on_newSearchButton_clicked();
      void on_uploadButton_clicked();
      void on_workspaceSettingsButton_clicked();
      void on_newThreadButton_clicked();
      void on_assistantChatsButton_clicked();
      void on_bottomChatButton_clicked();
      void on_bottomDocsButton_clicked();
      void on_bottomGithubButton_clicked();
      void on_bottomToolsButton_clicked();
      void on_headerSettingsButton_clicked();
      void on_toolsButton_clicked();
      void on_micButton_clicked();
      void on_sendButton_clicked();
      void on_createAgentButton_clicked();
      void on_editWorkspaceButton_clicked();
      void on_uploadFileButton_clicked();

  private:
      void setupStyleSheet();
      void setupLogo();
      void setupThreadList();

      Ui::MainWindow *ui;
  };

  #endif // MAINWINDOW_H
  ```

- [ ] 将 `hermind-desktop/mainwindow.cpp` 替换为以下完整内容：
  ```cpp
  #include "mainwindow.h"
  #include "ui_mainwindow.h"

  #include <QDebug>

  MainWindow::MainWindow(QWidget *parent)
      : QMainWindow(parent)
      , ui(new Ui::MainWindow)
  {
      ui->setupUi(this);

      setWindowTitle(tr("AnythingLLM | Superpowers for your OS using local AI"));

      setupLogo();
      setupThreadList();
      setupStyleSheet();

      connect(ui->popoutButton, &QToolButton::clicked, this, &MainWindow::on_popoutButton_clicked);
      connect(ui->newSearchButton, &QToolButton::clicked, this, &MainWindow::on_newSearchButton_clicked);
      connect(ui->uploadButton, &QToolButton::clicked, this, &MainWindow::on_uploadButton_clicked);
      connect(ui->workspaceSettingsButton, &QToolButton::clicked, this, &MainWindow::on_workspaceSettingsButton_clicked);
      connect(ui->newThreadButton, &QPushButton::clicked, this, &MainWindow::on_newThreadButton_clicked);
      connect(ui->assistantChatsButton, &QPushButton::clicked, this, &MainWindow::on_assistantChatsButton_clicked);
      connect(ui->bottomChatButton, &QToolButton::clicked, this, &MainWindow::on_bottomChatButton_clicked);
      connect(ui->bottomDocsButton, &QToolButton::clicked, this, &MainWindow::on_bottomDocsButton_clicked);
      connect(ui->bottomGithubButton, &QToolButton::clicked, this, &MainWindow::on_bottomGithubButton_clicked);
      connect(ui->bottomToolsButton, &QToolButton::clicked, this, &MainWindow::on_bottomToolsButton_clicked);
      connect(ui->headerSettingsButton, &QToolButton::clicked, this, &MainWindow::on_headerSettingsButton_clicked);
      connect(ui->toolsButton, &QPushButton::clicked, this, &MainWindow::on_toolsButton_clicked);
      connect(ui->micButton, &QToolButton::clicked, this, &MainWindow::on_micButton_clicked);
      connect(ui->sendButton, &QToolButton::clicked, this, &MainWindow::on_sendButton_clicked);
      connect(ui->createAgentButton, &QPushButton::clicked, this, &MainWindow::on_createAgentButton_clicked);
      connect(ui->editWorkspaceButton, &QPushButton::clicked, this, &MainWindow::on_editWorkspaceButton_clicked);
      connect(ui->uploadFileButton, &QPushButton::clicked, this, &MainWindow::on_uploadFileButton_clicked);
  }

  MainWindow::~MainWindow()
  {
      delete ui;
  }

  void MainWindow::setupLogo()
  {
      QPixmap logo(":/images/logo.svg");
      if (!logo.isNull()) {
          ui->logoLabel->setPixmap(logo.scaled(24, 24, Qt::KeepAspectRatio, Qt::SmoothTransformation));
      }
  }

  void MainWindow::setupThreadList()
  {
      ui->threadList->addItem("hello");
      ui->threadList->addItem("Thread");
      ui->threadList->addItem("hello");
      ui->threadList->addItem("联网搜索惠阳今天的...");
      ui->threadList->addItem("*New Thread");
      ui->threadList->setCurrentRow(4);
  }

  void MainWindow::setupStyleSheet()
  {
      setStyleSheet(R"(
          QMainWindow {
              background-color: #FFFFFF;
          }

          #sidebarFrame {
              background-color: #E8EDF2;
              border: none;
          }

          #chatFrame {
              background-color: #FFFFFF;
              border: none;
          }

          #brandLabel {
              color: #1F2937;
              font-size: 16px;
              font-weight: 600;
          }

          #workspaceLabel {
              color: #1F2937;
              font-size: 13px;
              font-weight: 500;
          }

          #workspaceIcon {
              color: #6B7280;
              font-size: 12px;
          }

          QToolButton {
              border: none;
              border-radius: 6px;
              background-color: transparent;
              color: #4B5563;
              font-size: 14px;
              padding: 4px 6px;
          }

          QToolButton:hover {
              background-color: #DDE3E9;
          }

          #popoutButton, #newSearchButton, #uploadButton, #workspaceSettingsButton,
          #headerSettingsButton, #micButton, #bottomChatButton, #bottomDocsButton,
          #bottomGithubButton, #bottomToolsButton {
              min-width: 28px;
              max-width: 28px;
              min-height: 28px;
              max-height: 28px;
              padding: 0px;
          }

          #sendButton {
              background-color: #5B8DEF;
              color: #FFFFFF;
              border-radius: 14px;
              min-width: 28px;
              max-width: 28px;
              min-height: 28px;
              max-height: 28px;
              padding: 0px;
          }

          #sendButton:hover {
              background-color: #4A7DE0;
          }

          #searchEdit {
              background-color: #FFFFFF;
              border: 1px solid #DDE3E9;
              border-radius: 8px;
              padding: 6px 10px;
              color: #1F2937;
              font-size: 13px;
          }

          #searchEdit::placeholder {
              color: #9CA3AF;
          }

          #threadList {
              background-color: transparent;
              border: none;
              outline: none;
          }

          #threadList::item {
              color: #374151;
              font-size: 13px;
              padding: 8px 12px;
              border-radius: 6px;
          }

          #threadList::item:selected {
              background-color: #D6E8F7;
              color: #1F2937;
          }

          #threadList::item:hover {
              background-color: #DFE5EA;
          }

          #newThreadButton, #assistantChatsButton {
              background-color: #FFFFFF;
              border: 1px solid #DDE3E9;
              border-radius: 8px;
              padding: 8px 12px;
              color: #1F2937;
              font-size: 13px;
              text-align: left;
          }

          #newThreadButton:hover, #assistantChatsButton:hover {
              background-color: #F8FAFC;
          }

          #workspaceNameLabel {
              color: #6B7280;
              font-size: 14px;
              font-weight: 500;
          }

          #versionLabel {
              color: #D4A017;
              font-size: 12px;
              font-weight: 500;
          }

          #welcomeLabel {
              color: #1F2937;
              font-size: 28px;
              font-weight: 500;
          }

          #inputFrame {
              background-color: #FFFFFF;
              border: 1px solid #E0E4E8;
              border-radius: 16px;
          }

          #messageEdit {
              background-color: transparent;
              border: none;
              color: #1F2937;
              font-size: 14px;
          }

          #messageEdit::placeholder {
              color: #9CA3AF;
          }

          #toolsButton {
              background-color: transparent;
              border: none;
              color: #6B7280;
              font-size: 13px;
              padding: 4px 8px;
          }

          #toolsButton:hover {
              background-color: #F1F3F5;
              border-radius: 6px;
          }

          #createAgentButton, #editWorkspaceButton, #uploadFileButton {
              background-color: #F1F3F5;
              border: none;
              border-radius: 16px;
              padding: 8px 16px;
              color: #374151;
              font-size: 13px;
          }

          #createAgentButton:hover, #editWorkspaceButton:hover, #uploadFileButton:hover {
              background-color: #E5E7EB;
          }

          #bottomChatButton, #bottomDocsButton, #bottomGithubButton, #bottomToolsButton {
              background-color: #DFE5EA;
              border-radius: 14px;
          }

          #bottomChatButton:hover, #bottomDocsButton:hover, #bottomGithubButton:hover, #bottomToolsButton:hover {
              background-color: #D2D8DE;
          }
      )");
  }

  void MainWindow::on_popoutButton_clicked() { qDebug() << "popout clicked"; }
  void MainWindow::on_newSearchButton_clicked() { qDebug() << "new search clicked"; }
  void MainWindow::on_uploadButton_clicked() { qDebug() << "upload clicked"; }
  void MainWindow::on_workspaceSettingsButton_clicked() { qDebug() << "workspace settings clicked"; }
  void MainWindow::on_newThreadButton_clicked() { qDebug() << "new thread clicked"; }
  void MainWindow::on_assistantChatsButton_clicked() { qDebug() << "assistant chats clicked"; }
  void MainWindow::on_bottomChatButton_clicked() { qDebug() << "bottom chat clicked"; }
  void MainWindow::on_bottomDocsButton_clicked() { qDebug() << "bottom docs clicked"; }
  void MainWindow::on_bottomGithubButton_clicked() { qDebug() << "bottom github clicked"; }
  void MainWindow::on_bottomToolsButton_clicked() { qDebug() << "bottom tools clicked"; }
  void MainWindow::on_headerSettingsButton_clicked() { qDebug() << "header settings clicked"; }
  void MainWindow::on_toolsButton_clicked() { qDebug() << "tools clicked"; }
  void MainWindow::on_micButton_clicked() { qDebug() << "mic clicked"; }
  void MainWindow::on_sendButton_clicked() { qDebug() << "send clicked"; }
  void MainWindow::on_createAgentButton_clicked() { qDebug() << "create agent clicked"; }
  void MainWindow::on_editWorkspaceButton_clicked() { qDebug() << "edit workspace clicked"; }
  void MainWindow::on_uploadFileButton_clicked() { qDebug() << "upload file clicked"; }
  ```

- [ ] 编译工程，确认无语法/连接错误：
  ```bash
  cd hermind-desktop
  qmake hermind-desktop.pro
  make
  ```
  预期：退出码为 `0`，生成可执行文件（Windows 下为 `debug/hermind-desktop.exe`，Linux/macOS 下为 `hermind-desktop`）。

- [ ] 运行可执行文件进行视觉验收：
  - Windows:
    ```bash
    cd hermind-desktop
    ./debug/hermind-desktop.exe
    ```
  - Linux/macOS:
    ```bash
    cd hermind-desktop
    ./hermind-desktop
    ```
  预期：窗口正常显示，无崩溃、无空白区域。

- [ ] 将运行窗口与截图 `D:\Pictures\bug-0028.png` 并排对比，逐项确认：
  | 检查项 | 预期观察 |
  |--------|----------|
  | 窗口标题栏 | 显示 `AnythingLLM \| Superpowers for your OS using local AI` |
  | 左侧边栏背景 | 浅蓝灰色（#E8EDF2） |
  | 品牌区 | 左上角显示 Hermind logo + `Hermind` 文字 |
  | 搜索框 | 白色圆角输入框，占位符 `搜索`，右侧有 `+` 按钮 |
  | 工作区标题 | 显示 `★ 我的工作区` + 上传/设置图标 |
  | 会话列表 | 显示 `hello`、`Thread`、`hello`、`联网搜索惠阳今天的...`、`*New Thread`，最后一项高亮蓝色 |
  | New Thread 按钮 | 白色圆角按钮，文字 `+  New Thread` |
  | Assistant Chats | 白色圆角按钮，文字 `☰  Assistant Chats` |
  | 底部图标按钮 | 四个圆形灰底按钮横向排列 |
  | 右侧头部 | 显示 `kb-big`、版本 `v1.14.0`、设置图标 |
  | 欢迎语 | 中央显示 `今天我能帮您什么？` |
  | 输入框 | 白色圆角大输入框，占位符 `发送消息`，底部有 `+  工具`、麦克风、蓝色发送按钮 |
  | 操作按钮 | 输入框下方横向排列 `创建代理`、`编辑工作区`、`上传文件` 三个灰色胶囊按钮 |

- [ ] 交互冒烟测试：依次点击侧边栏 `+ New Thread`、输入框下方 `创建代理`、右侧发送按钮，确认应用不崩溃；在终端/调试输出中看到对应的 `qDebug()` 日志。

- [ ] Commit：
  ```bash
  git add hermind-desktop/mainwindow.h hermind-desktop/mainwindow.cpp
  git commit -m "bug-0028: apply QSS styles and wire button slots"
  ```

## Self-Review

- [ ] 1. Spec-coverage table: `## Spec Coverage` 已将截图中所有可见元素映射到 Task 1–3；图标资源缺失标记为 `no-op`。
- [ ] 2. Placeholder scan: 正文中无 `TODO`/`TBD`/`implement later`/`similar to Task N`；所有代码片段完整可执行。
- [ ] 3. No phantom tasks: Task 1 新增资源文件并修改 `.pro`；Task 2 重写 `mainwindow.ui`；Task 3 重写 `.h`/`.cpp` 并完成编译/视觉验收；每个任务都有明确文件变更与验证步骤。
- [ ] 4. Dependency soundness: Task 2 依赖 Task 1 的资源路径；Task 3 依赖 Task 2 的控件 `objectName`；Task 3 不引用任何后续未定义符号。
- [ ] 5. Caller & build soundness: 本计划未修改跨文件共享的 C++ 签名/接口；唯一的“共享”约定是 `.ui` 中控件的 `objectName`，Task 3 的 QSS 选择器与 Task 2 的 `objectName` 已逐条对应；每个任务均要求 `qmake`/`make` 全工程编译通过。
- [ ] 6. Test-the-risk: UI 以视觉/交互验证替代单元测试；Task 3 明确要求运行应用并逐项对照截图验收；按钮点击不崩溃的冒烟测试覆盖运行时风险。
- [ ] 7. Type consistency: `mainwindow.h` 中声明的槽函数与 `mainwindow.cpp` 中的实现一一对应；`setupStyleSheet`/`setupLogo`/`setupThreadList` 辅助函数签名在 `.h` 与 `.cpp` 中一致；`.ui` 中控件类（`QToolButton`、`QPushButton`、`QListWidget` 等）与 `.cpp` 中使用的 Qt API 匹配。
