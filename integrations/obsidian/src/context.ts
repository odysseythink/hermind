import { App, MarkdownView } from "obsidian";
import { ObsidianContext } from "./api";

export function extractContext(app: App): ObsidianContext | undefined {
	const vaultPath = (app.vault.adapter as any).basePath as string | undefined;
	if (!vaultPath) {
		return undefined;
	}

	const ctx: ObsidianContext = { vault_path: vaultPath };

	const activeView = app.workspace.getActiveViewOfType(MarkdownView);
	if (activeView) {
		const file = activeView.file;
		if (file) {
			ctx.current_note = file.path;
		}
		const editor = activeView.editor;
		const selection = editor.getSelection();
		if (selection && selection.trim().length > 0) {
			ctx.selected_text = selection;
		}
		const cursor = editor.getCursor();
		ctx.cursor_line = cursor.line + 1;
	}

	return ctx;
}
