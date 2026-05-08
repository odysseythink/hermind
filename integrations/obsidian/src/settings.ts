import { App, PluginSettingTab, Setting } from "obsidian";
import HermindPlugin from "./main";

export interface HermindSettings {
	hermindUrl: string;
	autoAttachContext: boolean;
	saveFolder: string;
	showToolCalls: boolean;
	autoSave: boolean;
}

export const DEFAULT_SETTINGS: HermindSettings = {
	hermindUrl: "http://127.0.0.1:30000",
	autoAttachContext: true,
	saveFolder: "Hermind Conversations",
	showToolCalls: false,
	autoSave: false,
};

export class HermindSettingTab extends PluginSettingTab {
	plugin: HermindPlugin;

	constructor(app: App, plugin: HermindPlugin) {
		super(app, plugin);
		this.plugin = plugin;
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();
		containerEl.createEl("h2", { text: "Hermind Settings" });

		new Setting(containerEl)
			.setName("Hermind URL")
			.setDesc("Base URL of the running hermind web server")
			.addText((text) =>
				text
					.setPlaceholder("http://127.0.0.1:30000")
					.setValue(this.plugin.settings.hermindUrl)
					.onChange(async (value) => {
						this.plugin.settings.hermindUrl = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Auto-attach context")
			.setDesc("Automatically include current note and selection in messages")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.autoAttachContext)
					.onChange(async (value) => {
						this.plugin.settings.autoAttachContext = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Save folder")
			.setDesc("Default folder for saved conversations")
			.addText((text) =>
				text
					.setPlaceholder("Hermind Conversations")
					.setValue(this.plugin.settings.saveFolder)
					.onChange(async (value) => {
						this.plugin.settings.saveFolder = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Show tool calls")
			.setDesc("Expand tool call details in chat messages")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.showToolCalls)
					.onChange(async (value) => {
						this.plugin.settings.showToolCalls = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Auto-save conversation")
			.setDesc("Automatically save conversation to a note after each reply")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.autoSave)
					.onChange(async (value) => {
						this.plugin.settings.autoSave = value;
						await this.plugin.saveSettings();
					})
			);
	}
}
