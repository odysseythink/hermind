# MainSettingWidget 界面复刻实施计划

**Goal:** 根据截图 `D:\\Pictures\\bug-0028.png` 在 `hermind-desktop/main_setting_widget.h`、`.cpp`、`.ui` 三个文件中复刻设置界面（外观/界面偏好设置页），并通过编译与视觉对照验证。

**Architecture:** 在 `main_setting_widget.ui` 中用 Qt Designer XML 构建左右分栏布局：左侧 260px 浅蓝灰色侧边栏包含 Logo/品牌、设置分组菜单、支持链接和底部图标按钮；右侧白色圆角内容卡片展示“界面偏好设置”标题、副标题、分隔线和三个下拉设置项。样式通过 `main_setting_widget.cpp` 中的 `setStyleSheet` 统一注入，交互通过 QPushButton/QComboBox 信号槽连接。

**Tech Stack:** Qt 6/5 Widgets (C++17), qmake (.pro), Qt Designer UI XML.

> For executing workers: implement this plan task-by-task (prefer a fresh subagent/Task per task — a clean context per task avoids single-session degradation). Steps use - [ ] checkboxes for tracking.

## File Structure

| 文件 | 职责 |
|---|---|
| `hermind-desktop/main_setting_widget.ui` | 完整 UI 布局：侧边栏、菜单、内容区、下拉框 |
| `hermind-desktop/main_setting_widget.h` | 类声明、私有辅助函数与槽函数声明 |
| `hermind-desktop/main_setting_widget.cpp` | 构造函数、样式表、信号槽连接、槽实现 |
| `hermind-desktop/hermind-desktop.pro` | 现有工程文件（不修改，仅用于构建验证） |

## Dependency Overview

```text
Task 1 (.ui 布局) ─┬─► Task 2 (.h 头文件) ─┬─► Task 3 (.cpp 实现)
                   │                       │
                   └───────────────────────┘
                                           ▼
                                    Task 4 (构建验证)
                                           ▼
                                    Task 5 (视觉对照验证)
```

- Task 1 不依赖其他任务，最先执行。
- Task 2 依赖 Task 1（头文件中的 `Ui::MainSettingWidget` 由 .ui 生成，需确认对象名）。
- Task 3 依赖 Task 1 和 Task 2（引用 ui 成员和槽函数）。
- Task 4 依赖 Task 1-3（完整构建）。
- Task 5 依赖 Task 4（运行可执行文件）。

## Risks & Open Questions

- 项目没有独立的 Qt 单元测试框架；UI 正确性以“编译通过 + 运行截图对比”为准。
- 截图中的图标为 AnythingLLM 图标资源，当前工程只有 `assets/logo.svg`，无其他图标文件，因此使用 Unicode 符号近似。
- 截图标题栏显示 “AnythingLLM”，但 `main_chat_widget.ui` 中品牌文本为 “Hermind”。本计划复刻截图时在 `brandLabel` 使用 “AnythingLLM”；若项目要求统一为 “Hermind”，可在 Task 3 中修改一行文本。

### Task 1: 编写 `main_setting_widget.ui` 完整布局

**Depends on:** none

**Files:**
- Create: `hermind-desktop/main_setting_widget.ui`（覆盖现有空 UI）

**说明：** 用 Qt Designer XML 实现左右分栏。左侧 sidebar 固定 260px，右侧 contentFrame 为白色圆角卡片。所有对象名必须与 Task 3 样式表和槽函数引用一致。

- [ ] 将以下完整 XML 写入 `hermind-desktop/main_setting_widget.ui`（覆盖原文件）：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ui version="4.0">
 <class>MainSettingWidget</class>
 <widget class="QWidget" name="MainSettingWidget">
  <property name="geometry">
   <rect>
    <x>0</x>
    <y>0</y>
    <width>1200</width>
    <height>800</height>
   </rect>
  </property>
  <property name="windowTitle">
   <string>Form</string>
  </property>
  <layout class="QHBoxLayout" name="horizontalLayout">
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
     <property name="frameShape">
      <enum>QFrame::NoFrame</enum>
     </property>
     <layout class="QVBoxLayout" name="verticalLayout">
      <property name="spacing">
       <number>4</number>
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
           <string>AnythingLLM</string>
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
       </layout>
      </item>
      <item>
       <widget class="QLabel" name="settingsSectionLabel">
        <property name="text">
         <string>设置</string>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="aiProviderButton">
        <property name="text">
         <string>🤖  人工智能提供商   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="adminButton">
        <property name="text">
         <string>⚙  管理员   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="agentSkillsButton">
        <property name="text">
         <string>✨  代理技能   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="meetingAssistantButton">
        <property name="text">
         <string>📊  会议助理</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="desktopAssistantButton">
        <property name="text">
         <string>🖥  桌面助手</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="communityCenterButton">
        <property name="text">
         <string>⊞  社区中心   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="appearanceButton">
        <property name="text">
         <string>🎨  外观   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
        <property name="checked">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="channelsButton">
        <property name="text">
         <string>📡  频道   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QPushButton" name="toolsButton">
        <property name="text">
         <string>🧰  工具   ›</string>
        </property>
        <property name="flat">
         <bool>true</bool>
        </property>
        <property name="checkable">
         <bool>true</bool>
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
       <widget class="QLabel" name="supportLabel">
        <property name="text">
         <string>联系支持</string>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QLabel" name="privacyLabel">
        <property name="text">
         <string>隐私与数据</string>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QLabel" name="termsLabel">
        <property name="text">
         <string>条款与许可</string>
        </property>
       </widget>
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
    <widget class="QFrame" name="contentFrame">
     <property name="frameShape">
      <enum>QFrame::NoFrame</enum>
     </property>
     <layout class="QVBoxLayout" name="contentLayout">
      <property name="spacing">
       <number>24</number>
      </property>
      <property name="leftMargin">
       <number>32</number>
      </property>
      <property name="topMargin">
       <number>32</number>
      </property>
      <property name="rightMargin">
       <number>32</number>
      </property>
      <property name="bottomMargin">
       <number>32</number>
      </property>
      <item>
       <widget class="QLabel" name="titleLabel">
        <property name="text">
         <string>界面偏好设置</string>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QLabel" name="subtitleLabel">
        <property name="text">
         <string>设置您的 AnythingLLM 界面偏好。</string>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QFrame" name="separatorLine">
        <property name="frameShape">
         <enum>QFrame::HLine</enum>
        </property>
        <property name="frameShadow">
         <enum>QFrame::Plain</enum>
        </property>
       </widget>
      </item>
      <item>
       <widget class="QFrame" name="defaultWindowRow">
        <property name="frameShape">
         <enum>QFrame::NoFrame</enum>
        </property>
        <layout class="QVBoxLayout" name="defaultWindowLayout">
         <property name="spacing">
          <number>4</number>
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
          <widget class="QLabel" name="defaultWindowLabel">
           <property name="text">
            <string>默认窗口</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QLabel" name="defaultWindowDesc">
           <property name="text">
            <string>设置应用程序启动时默认显示的窗口。</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QComboBox" name="defaultWindowCombo">
           <item>
            <property name="text">
             <string>主界面</string>
            </property>
           </item>
          </widget>
         </item>
        </layout>
       </widget>
      </item>
      <item>
       <widget class="QFrame" name="themeRow">
        <property name="frameShape">
         <enum>QFrame::NoFrame</enum>
        </property>
        <layout class="QVBoxLayout" name="themeLayout">
         <property name="spacing">
          <number>4</number>
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
          <widget class="QLabel" name="themeLabel">
           <property name="text">
            <string>主题</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QLabel" name="themeDesc">
           <property name="text">
            <string>选择您偏好的应用配色主题。</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QComboBox" name="themeCombo">
           <item>
            <property name="text">
             <string>System</string>
            </property>
           </item>
           <item>
            <property name="text">
             <string>Light</string>
            </property>
           </item>
           <item>
            <property name="text">
             <string>Dark</string>
            </property>
           </item>
          </widget>
         </item>
        </layout>
       </widget>
      </item>
      <item>
       <widget class="QFrame" name="languageRow">
        <property name="frameShape">
         <enum>QFrame::NoFrame</enum>
        </property>
        <layout class="QVBoxLayout" name="languageLayout">
         <property name="spacing">
          <number>4</number>
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
          <widget class="QLabel" name="languageLabel">
           <property name="text">
            <string>显示语言</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QLabel" name="languageDesc">
           <property name="text">
            <string>选择显示 AnythingLLM 界面所用的语言（若有翻译可用）。</string>
           </property>
          </widget>
         </item>
         <item>
          <widget class="QComboBox" name="languageCombo">
           <item>
            <property name="text">
             <string>English</string>
            </property>
           </item>
           <item>
            <property name="text">
             <string>中文（简体）</string>
            </property>
           </item>
          </widget>
         </item>
        </layout>
       </widget>
      </item>
      <item>
       <spacer name="contentSpacer">
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
 <resources/>
 <connections/>
</ui>
```

- [ ] 保存文件后运行 `uic` 验证 XML 可被解析（不生成对象代码，仅检查格式）：
  - Windows (Git Bash): `cd hermind-desktop && uic main_setting_widget.ui -o /dev/null`
  - 预期：命令返回 0，无错误输出。
- [ ] Commit: `git add hermind-desktop/main_setting_widget.ui && git commit -m "Task 1: add MainSettingWidget UI layout"`

### Task 2: 编写 `main_setting_widget.h` 类声明与槽函数

**Depends on:** Task 1

**Files:**
- Modify: `hermind-desktop/main_setting_widget.h`（覆盖现有 22 行）

**说明：** 头文件前向声明 `Ui::MainSettingWidget`（由 Task 1 的 .ui 生成），声明所有菜单按钮、底部图标按钮、下拉框的槽函数，以及样式/连接辅助函数。

- [ ] 将以下完整头文件写入 `hermind-desktop/main_setting_widget.h`：

```cpp
#ifndef MAIN_SETTING_WIDGET_H
#define MAIN_SETTING_WIDGET_H

#include <QWidget>

namespace Ui {
class MainSettingWidget;
}

class MainSettingWidget : public QWidget
{
    Q_OBJECT

public:
    explicit MainSettingWidget(QWidget *parent = nullptr);
    ~MainSettingWidget() override;

private slots:
    // 左侧设置菜单
    void on_aiProviderButton_clicked();
    void on_adminButton_clicked();
    void on_agentSkillsButton_clicked();
    void on_meetingAssistantButton_clicked();
    void on_desktopAssistantButton_clicked();
    void on_communityCenterButton_clicked();
    void on_appearanceButton_clicked();
    void on_channelsButton_clicked();
    void on_toolsButton_clicked();

    // 底部图标按钮
    void on_bottomChatButton_clicked();
    void on_bottomDocsButton_clicked();
    void on_bottomGithubButton_clicked();
    void on_bottomToolsButton_clicked();

    // 右侧下拉框变更
    void on_defaultWindowCombo_currentIndexChanged(int index);
    void on_themeCombo_currentIndexChanged(int index);
    void on_languageCombo_currentIndexChanged(int index);

private:
    void setupLogo();
    void setupStyleSheet();
    void setupMenuGroup();
    void setupConnections();

    Ui::MainSettingWidget *ui;
};

#endif // MAIN_SETTING_WIDGET_H
```

- [ ] 用 `grep` 确认新头文件没有引用未在 .ui 中声明的对象名：
  - `grep -n "ui->" hermind-desktop/main_setting_widget.h`
  - 预期：无输出（头文件不直接访问 ui 成员）。
- [ ] Commit: `git add hermind-desktop/main_setting_widget.h && git commit -m "Task 2: add MainSettingWidget header with slots"`

### Task 3: 编写 `main_setting_widget.cpp` 实现、样式与信号连接

**Depends on:** Task 1, Task 2

**Files:**
- Modify: `hermind-desktop/main_setting_widget.cpp`（覆盖现有 14 行）

**说明：** 完成构造函数、Logo 加载、菜单按钮互斥组、所有信号槽连接、完整样式表，以及所有槽函数占位实现（输出 qDebug 便于后续接入业务）。

- [ ] 将以下完整 `.cpp` 写入 `hermind-desktop/main_setting_widget.cpp`：

```cpp
#include "main_setting_widget.h"
#include "ui_main_setting_widget.h"

#include <QButtonGroup>
#include <QDebug>
#include <QPixmap>

MainSettingWidget::MainSettingWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainSettingWidget)
{
    ui->setupUi(this);

    setupLogo();
    setupMenuGroup();
    setupConnections();
    setupStyleSheet();
}

MainSettingWidget::~MainSettingWidget()
{
    delete ui;
}

void MainSettingWidget::setupLogo()
{
    QPixmap logo(":/images/logo.svg");
    if (!logo.isNull()) {
        ui->logoLabel->setPixmap(logo.scaled(24, 24, Qt::KeepAspectRatio, Qt::SmoothTransformation));
    }
}

void MainSettingWidget::setupMenuGroup()
{
    QButtonGroup *menuGroup = new QButtonGroup(this);
    menuGroup->setExclusive(true);
    menuGroup->addButton(ui->aiProviderButton);
    menuGroup->addButton(ui->adminButton);
    menuGroup->addButton(ui->agentSkillsButton);
    menuGroup->addButton(ui->meetingAssistantButton);
    menuGroup->addButton(ui->desktopAssistantButton);
    menuGroup->addButton(ui->communityCenterButton);
    menuGroup->addButton(ui->appearanceButton);
    menuGroup->addButton(ui->channelsButton);
    menuGroup->addButton(ui->toolsButton);

    ui->appearanceButton->setChecked(true);
}

void MainSettingWidget::setupConnections()
{
    connect(ui->aiProviderButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_aiProviderButton_clicked);
    connect(ui->adminButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_adminButton_clicked);
    connect(ui->agentSkillsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_agentSkillsButton_clicked);
    connect(ui->meetingAssistantButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_meetingAssistantButton_clicked);
    connect(ui->desktopAssistantButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_desktopAssistantButton_clicked);
    connect(ui->communityCenterButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_communityCenterButton_clicked);
    connect(ui->appearanceButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_appearanceButton_clicked);
    connect(ui->channelsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_channelsButton_clicked);
    connect(ui->toolsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_toolsButton_clicked);

    connect(ui->bottomChatButton, &QToolButton::clicked,
            this, &MainSettingWidget::on_bottomChatButton_clicked);
    connect(ui->bottomDocsButton, &QToolButton::clicked,
            this, &MainSettingWidget::on_bottomDocsButton_clicked);
    connect(ui->bottomGithubButton, &QToolButton::clicked,
            this, &MainSettingWidget::on_bottomGithubButton_clicked);
    connect(ui->bottomToolsButton, &QToolButton::clicked,
            this, &MainSettingWidget::on_bottomToolsButton_clicked);

    connect(ui->defaultWindowCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_defaultWindowCombo_currentIndexChanged);
    connect(ui->themeCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_themeCombo_currentIndexChanged);
    connect(ui->languageCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_languageCombo_currentIndexChanged);
}

void MainSettingWidget::setupStyleSheet()
{
    setStyleSheet(R"(
        MainSettingWidget {
            background-color: #FFFFFF;
        }

        #sidebarFrame {
            background-color: #E8EDF2;
            border: none;
        }

        #brandLabel {
            color: #1F2937;
            font-size: 16px;
            font-weight: 600;
        }

        #settingsSectionLabel {
            color: #6B7280;
            font-size: 12px;
            font-weight: 500;
            margin-top: 8px;
        }

        #sidebarFrame QPushButton {
            text-align: left;
            border: none;
            border-radius: 8px;
            background-color: transparent;
            color: #374151;
            font-size: 14px;
            padding: 10px 12px;
        }

        #sidebarFrame QPushButton:hover {
            background-color: #DDE3E9;
        }

        #sidebarFrame QPushButton:checked {
            background-color: #D6E8F7;
            color: #1F2937;
        }

        #supportLabel, #privacyLabel, #termsLabel {
            color: #6B7280;
            font-size: 12px;
            padding: 2px 12px;
        }

        #bottomChatButton, #bottomDocsButton, #bottomGithubButton, #bottomToolsButton {
            border: none;
            border-radius: 14px;
            background-color: #DFE5EA;
            color: #4B5563;
            min-width: 28px;
            max-width: 28px;
            min-height: 28px;
            max-height: 28px;
            padding: 0px;
        }

        #bottomChatButton:hover, #bottomDocsButton:hover, #bottomGithubButton:hover, #bottomToolsButton:hover {
            background-color: #D2D8DE;
        }

        #contentFrame {
            background-color: #FFFFFF;
            border: 1px solid #E5E7EB;
            border-radius: 16px;
        }

        #titleLabel {
            color: #1F2937;
            font-size: 22px;
            font-weight: 600;
        }

        #subtitleLabel {
            color: #6B7280;
            font-size: 13px;
        }

        #separatorLine {
            color: #E5E7EB;
            background-color: #E5E7EB;
            border: none;
            max-height: 1px;
        }

        #defaultWindowLabel, #themeLabel, #languageLabel {
            color: #1F2937;
            font-size: 14px;
            font-weight: 500;
        }

        #defaultWindowDesc, #themeDesc, #languageDesc {
            color: #6B7280;
            font-size: 12px;
        }

        QComboBox {
            background-color: #F1F3F5;
            border: 1px solid #E5E7EB;
            border-radius: 8px;
            padding: 8px 12px;
            color: #1F2937;
            font-size: 13px;
            min-width: 140px;
            max-width: 220px;
        }

        QComboBox:hover {
            border-color: #D1D5DB;
        }

        QComboBox::drop-down {
            border: none;
            width: 24px;
        }

        QComboBox QAbstractItemView {
            background-color: #FFFFFF;
            border: 1px solid #E5E7EB;
            selection-background-color: #D6E8F7;
        }
    )");
}

void MainSettingWidget::on_aiProviderButton_clicked() { qDebug() << "ai provider clicked"; }
void MainSettingWidget::on_adminButton_clicked() { qDebug() << "admin clicked"; }
void MainSettingWidget::on_agentSkillsButton_clicked() { qDebug() << "agent skills clicked"; }
void MainSettingWidget::on_meetingAssistantButton_clicked() { qDebug() << "meeting assistant clicked"; }
void MainSettingWidget::on_desktopAssistantButton_clicked() { qDebug() << "desktop assistant clicked"; }
void MainSettingWidget::on_communityCenterButton_clicked() { qDebug() << "community center clicked"; }
void MainSettingWidget::on_appearanceButton_clicked() { qDebug() << "appearance clicked"; }
void MainSettingWidget::on_channelsButton_clicked() { qDebug() << "channels clicked"; }
void MainSettingWidget::on_toolsButton_clicked() { qDebug() << "tools clicked"; }

void MainSettingWidget::on_bottomChatButton_clicked() { qDebug() << "bottom chat clicked"; }
void MainSettingWidget::on_bottomDocsButton_clicked() { qDebug() << "bottom docs clicked"; }
void MainSettingWidget::on_bottomGithubButton_clicked() { qDebug() << "bottom github clicked"; }
void MainSettingWidget::on_bottomToolsButton_clicked() { qDebug() << "bottom tools clicked"; }

void MainSettingWidget::on_defaultWindowCombo_currentIndexChanged(int index)
{
    qDebug() << "default window changed:" << index;
}

void MainSettingWidget::on_themeCombo_currentIndexChanged(int index)
{
    qDebug() << "theme changed:" << index;
}

void MainSettingWidget::on_languageCombo_currentIndexChanged(int index)
{
    qDebug() << "language changed:" << index;
}
```

- [ ] 用 `grep` 确认 `.cpp` 中引用的所有 `ui->` 成员均存在于 Task 1 的 .ui 中：
  - `grep -oP 'ui->[a-zA-Z0-9_]+' hermind-desktop/main_setting_widget.cpp | sort -u`
  - 预期输出包含：aiProviderButton, appearanceButton, bottomChatButton, bottomDocsButton, bottomGithubButton, bottomToolsButton, channelsButton, communityCenterButton, defaultWindowCombo, desktopAssistantButton, languageCombo, logoLabel, meetingAssistantButton, themeCombo, toolsButton
  - 然后与 .ui 中的对象名对比：`grep -oP 'name="\K[a-zA-Z0-9_]+' hermind-desktop/main_setting_widget.ui | sort -u | grep -E 'Button|Combo|Label'`
- [ ] Commit: `git add hermind-desktop/main_setting_widget.cpp && git commit -m "Task 3: add MainSettingWidget implementation, stylesheet and connections"`

### Task 4: 构建验证

**Depends on:** Task 1, Task 2, Task 3

**Files:**
- Test: 使用 `hermind-desktop/hermind-desktop.pro`（不修改）
- Build outputs: `hermind-desktop/debug/hermind-desktop.exe` 或 `hermind-desktop/release/hermind-desktop.exe`（取决于工具链）

**说明：** 运行 qmake 重新生成 Makefile，再执行编译，确保三个修改的文件无语法/连接错误。

- [ ] 进入桌面工程目录并重新生成 Makefile：
  - `cd hermind-desktop && qmake hermind-desktop.pro`
  - 预期：命令返回 0，无错误输出。
- [ ] 编译：
  - Windows (MinGW): `mingw32-make`
  - Windows (MSVC / nmake): `nmake`
  - macOS/Linux: `make`
  - 预期：编译过程无 error，最终输出类似 `linking debug/hermind-desktop.exe` 或 `linking release/hermind-desktop.exe`。
- [ ] 确认可执行文件存在：
  - Windows: `ls hermind-desktop/debug/hermind-desktop.exe`
  - 其他平台: `ls hermind-desktop/hermind-desktop` 或 `ls hermind-desktop/debug/hermind-desktop`
  - 预期：文件存在。
- [ ] 在终端记录构建结果（无需提交源码变更，构建产物由验证步骤本身证明正确性）。

### Task 5: 视觉对照验证

**Depends on:** Task 4

**Files:**
- 临时修改: `hermind-desktop/main.cpp`（验证后必须还原，不提交）

**说明：** 临时把入口改为直接显示 `MainSettingWidget`，运行后与截图 `D:\Pictures\bug-0028.png` 逐项对照，确认还原后无残留变更。

- [ ] 临时修改 `hermind-desktop/main.cpp` 为以下内容（仅用于本地验证）：

```cpp
#include <QApplication>
#include "main_setting_widget.h"

int main(int argc, char *argv[])
{
    QApplication a(argc, argv);
    MainSettingWidget w;
    w.setWindowTitle(QString::fromUtf8("AnythingLLM | Superpowers for your OS using local AI"));
    w.resize(1200, 800);
    w.show();
    return a.exec();
}
```

- [ ] 重新编译：`cd hermind-desktop && qmake hermind-desktop.pro && make`（或对应平台的构建命令）。
- [ ] 运行可执行文件，窗口最大化或保持 1200x800，与截图逐项核对：
  - 左侧边栏背景为浅蓝灰色，宽度约 260px。
  - 顶部显示 Logo + “AnythingLLM”。
  - “设置”分组下依次出现：人工智能提供商、管理员、代理技能、会议助理、桌面助手、社区中心、外观（高亮选中）、频道、工具。
  - 底部出现：联系支持、隐私与数据、条款与许可，以及四个圆形图标按钮。
  - 右侧内容卡片为白色圆角矩形，顶部标题为“界面偏好设置”，副标题为“设置您的 AnythingLLM 界面偏好。”。
  - 分隔线下方依次出现：默认窗口（下拉“主界面”）、主题（下拉“System”）、显示语言（下拉“English”）。
  - 下拉框可点击展开，菜单按钮可点击切换选中状态。
- [ ] 使用截图工具截取运行效果，与 `D:\Pictures\bug-0028.png` 并排放置对比；差异处记录到 `hermind-desktop/.setting_ui_diff.md`（如肉眼无显著差异则写“无显著差异”）。
- [ ] 还原 `hermind-desktop/main.cpp` 为原始内容（即重新构造 `MainWindow`）。
- [ ] 用 `git status` 确认工作区干净：
  - `git status --short`
  - 预期：无 `main.cpp` 变更，仅有已提交的三个文件。
- [ ] 视觉验证完成；无需额外提交源码变更。

## Self-Review

- [ ] 1. **Spec-coverage table:**

| 截图要求 | 覆盖任务 | 状态 |
|---|---|---|
| 左侧边栏浅蓝灰背景、固定 260px 宽度 | Task 1, Task 3 | covered |
| Logo + 品牌文本 | Task 1, Task 3 | covered |
| “设置”分组标题 | Task 1, Task 3 | covered |
| 菜单项：人工智能提供商、管理员、代理技能、会议助理、桌面助手、社区中心、外观（默认选中）、频道、工具 | Task 1, Task 2, Task 3 | covered |
| 支持链接：联系支持、隐私与数据、条款与许可 | Task 1, Task 3 | covered |
| 底部四个圆形图标按钮 | Task 1, Task 2, Task 3 | covered |
| 右侧白色圆角内容卡片 | Task 1, Task 3 | covered |
| 标题“界面偏好设置”及副标题 | Task 1, Task 3 | covered |
| 分隔线 | Task 1, Task 3 | covered |
| 默认窗口下拉框（默认“主界面”） | Task 1, Task 2, Task 3 | covered |
| 主题下拉框（默认“System”） | Task 1, Task 2, Task 3 | covered |
| 显示语言下拉框（默认“English”） | Task 1, Task 2, Task 3 | covered |
| 颜色、间距、字体、圆角等视觉样式 | Task 3 | covered |
| 编译通过 | Task 4 | covered |
| 与截图视觉对照 | Task 5 | covered |

- [ ] 2. **Placeholder scan:** 计划中无 TODO/TBD/“稍后实现”/死代码；所有步骤均给出完整代码或明确命令。
- [ ] 3. **No phantom tasks:** 每个任务均产生可验证变更：Task 1-3 产生源码文件修改并提交；Task 4 产生成功构建；Task 5 产生视觉对照结果；无 `--allow-empty` 提交。
- [ ] 4. **Dependency soundness:** Task 1 无依赖；Task 2 依赖 Task 1（确认 .ui 对象名）；Task 3 依赖 Task 1、Task 2；Task 4 依赖 Task 1-3；Task 5 依赖 Task 4。无向前引用。
- [ ] 5. **Caller & build soundness:** 未修改任何公共签名/接口；`MainSettingWidget` 构造函数/析构函数签名保持不变，无调用方需要更新。构建验证由 Task 4 完成（qmake + make，全工程编译）。
- [ ] 6. **Test-the-risk:** UI 属于非自动化测试代码；风险通过 Task 4（编译是否通过）和 Task 5（视觉对照）覆盖。布局与对象名一致性由 Task 3 的 `grep` 检查保证。
- [ ] 7. **Type consistency:** `.ui` 中声明的对象名（`aiProviderButton`、`defaultWindowCombo` 等）与 `.h` 槽名、`.cpp` 中的 `ui->xxx` 引用完全一致；所有类型均为 Qt 标准控件类型，无自定义类型变更。
