import { Plugin, WorkspaceLeaf, MarkdownView } from "obsidian";
import { HermindSettings, DEFAULT_SETTINGS, HermindSettingTab } from "./settings";
import { ChatView, VIEW_TYPE_HERMIND } from "./chat/ChatView";

export default class HermindPlugin extends Plugin {
	settings: HermindSettings;

	async onload(): Promise<void> {
		await this.loadSettings();

		this.registerView(VIEW_TYPE_HERMIND, (leaf) => new ChatView(leaf, this.settings));

		this.addRibbonIcon("message-circle", "Open Hermind Chat", () => {
			this.activateView();
		});

		this.addCommand({
			id: "open-hermind-chat",
			name: "Open Hermind Chat",
			callback: () => this.activateView(),
		});

		this.addCommand({
			id: "send-selection-to-hermind",
			name: "Send Selection to Hermind",
			callback: () => {
				const view = this.app.workspace.getActiveViewOfType(MarkdownView);
				if (!view) return;
				const selection = view.editor.getSelection();
				if (!selection) return;
				this.activateView().then(() => {
					const leaf = this.app.workspace.getLeavesOfType(VIEW_TYPE_HERMIND)[0];
					if (leaf && leaf.view instanceof ChatView) {
						leaf.view.sendSelection(selection);
					}
				});
			},
		});

		this.addCommand({
			id: "save-hermind-conversation",
			name: "Save Hermind Conversation",
			callback: () => {
				const leaf = this.app.workspace.getLeavesOfType(VIEW_TYPE_HERMIND)[0];
				if (leaf && leaf.view instanceof ChatView) {
					leaf.view.saveConversation();
				}
			},
		});

		this.addSettingTab(new HermindSettingTab(this.app, this));
	}

	onunload(): void {
		this.app.workspace.detachLeavesOfType(VIEW_TYPE_HERMIND);
	}

	async loadSettings(): Promise<void> {
		this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
	}

	async saveSettings(): Promise<void> {
		await this.saveData(this.settings);
	}

	private async activateView(): Promise<void> {
		const { workspace } = this.app;
		let leaf = workspace.getLeavesOfType(VIEW_TYPE_HERMIND)[0];
		if (!leaf) {
			leaf = workspace.getRightLeaf(false) ?? workspace.getLeaf(true);
			await leaf.setViewState({ type: VIEW_TYPE_HERMIND, active: true });
		}
		workspace.revealLeaf(leaf);
	}
}
