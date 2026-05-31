import { useEffect, useState } from "react";
import Sidebar from "@/components/SettingsSidebar";
import { isMobile } from "react-device-detect";
import Admin from "@/models/admin";
import AgentSkills from "@/models/agentSkills";
import Workspace from "@/models/workspace";
import showToast from "@/utils/toast";
import {
  Plus,
  Pencil,
  Trash,
  Eye,
  PushPin,
  PushPinSlash,
  Archive,
  ArrowCounterClockwise,
  BookOpen,
  X,
} from "@phosphor-icons/react";

const STATUS_STYLES = {
  active: "bg-green-500/20 text-green-400",
  stale: "bg-yellow-500/20 text-yellow-400",
  archived: "bg-gray-500/20 text-gray-400",
};

export default function AgentCreatedSkillsPage() {
  const [workspaces, setWorkspaces] = useState([]);
  const [selectedWsSlug, setSelectedWsSlug] = useState("");
  const [skills, setSkills] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [editingSkill, setEditingSkill] = useState(null);
  const [viewingSkill, setViewingSkill] = useState(null);

  useEffect(() => {
    async function fetchWorkspaces() {
      try {
        const { workspaces: wss } = await Admin.workspaces();
        setWorkspaces(wss || []);
        if (wss?.length > 0) {
          setSelectedWsSlug(wss[0].slug);
        }
      } catch (e) {
        showToast("Failed to load workspaces", "error");
      }
      setLoading(false);
    }
    fetchWorkspaces();
  }, []);

  useEffect(() => {
    if (!selectedWsSlug) return;
    async function fetchSkills() {
      try {
        const res = await AgentSkills.list(selectedWsSlug, true);
        setSkills(res.skills || []);
      } catch (e) {
        showToast("Failed to load skills", "error");
      }
    }
    fetchSkills();
  }, [selectedWsSlug]);

  const handleDelete = async (skill) => {
    if (!window.confirm(`Delete skill "${skill.name}"? This cannot be undone.`))
      return;
    try {
      await AgentSkills.delete(selectedWsSlug, skill.slug);
      showToast("Skill deleted", "success");
      setSkills((prev) => prev.filter((s) => s.slug !== skill.slug));
    } catch (e) {
      showToast(e.message || "Failed to delete", "error");
    }
  };

  const handleTogglePin = async (skill) => {
    try {
      await AgentSkills.update(selectedWsSlug, skill.slug, {
        pinned: !skill.pinned,
      });
      setSkills((prev) =>
        prev.map((s) =>
          s.slug === skill.slug ? { ...s, pinned: !s.pinned } : s
        )
      );
    } catch (e) {
      showToast(e.message || "Failed to update pin", "error");
    }
  };

  const handleToggleStatus = async (skill, newStatus) => {
    try {
      await AgentSkills.update(selectedWsSlug, skill.slug, {
        status: newStatus,
      });
      setSkills((prev) =>
        prev.map((s) =>
          s.slug === skill.slug ? { ...s, status: newStatus } : s
        )
      );
    } catch (e) {
      showToast(e.message || "Failed to update status", "error");
    }
  };

  const refreshSkills = async () => {
    try {
      const res = await AgentSkills.list(selectedWsSlug, true);
      setSkills(res.skills || []);
    } catch (e) {
      showToast("Failed to refresh skills", "error");
    }
  };

  return (
    <div className="w-screen h-screen overflow-hidden bg-theme-bg-container flex">
      <Sidebar />
      <div
        style={{ height: isMobile ? "100%" : "calc(100% - 32px)" }}
        className="relative md:ml-[2px] md:mr-[16px] md:my-[16px] md:rounded-[16px] bg-theme-bg-secondary w-full h-full overflow-y-scroll p-4 md:p-0"
      >
        <div className="flex flex-col w-full px-1 md:pl-6 md:pr-[50px] md:py-6 py-16">
          <div className="w-full flex flex-col gap-y-1 pb-6 border-white/10 border-b-2">
            <div className="items-center flex gap-x-4 justify-between">
              <div>
                <p className="text-lg leading-6 font-bold text-theme-text-primary">
                  Agent-Created Skills
                </p>
                <p className="text-xs leading-[18px] font-base text-theme-text-secondary mt-1">
                  Procedural memory skills created by the agent or users.
                  Skills are scoped per workspace.
                </p>
              </div>
              <button
                onClick={() => setShowCreate(true)}
                className="flex items-center gap-x-2 bg-cta-button hover:bg-cta-button-hover text-white px-4 py-2 rounded-lg text-sm transition-colors"
              >
                <Plus size={18} />
                New Skill
              </button>
            </div>
          </div>

          {/* Workspace selector */}
          <div className="mt-4 mb-4">
            <label className="text-sm text-theme-text-secondary">Workspace</label>
            <select
              className="mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary w-full md:w-80"
              value={selectedWsSlug}
              onChange={(e) => setSelectedWsSlug(e.target.value)}
            >
              {workspaces.map((ws) => (
                <option key={ws.slug} value={ws.slug}>
                  {ws.name}
                </option>
              ))}
            </select>
          </div>

          {loading ? (
            <div className="text-theme-text-secondary text-sm">Loading...</div>
          ) : skills.length === 0 ? (
            <div className="text-theme-text-secondary text-sm mt-8">
              No skills found in this workspace. The agent will create skills
              automatically when it learns reusable workflows.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-white/10 text-theme-text-secondary">
                    <th className="py-3 px-2">Name</th>
                    <th className="py-3 px-2">Category</th>
                    <th className="py-3 px-2">Status</th>
                    <th className="py-3 px-2">Uses</th>
                    <th className="py-3 px-2">Last Used</th>
                    <th className="py-3 px-2">Created By</th>
                    <th className="py-3 px-2 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {skills.map((skill) => (
                    <tr
                      key={skill.slug}
                      className="border-b border-white/5 hover:bg-white/5 transition-colors"
                    >
                      <td className="py-3 px-2">
                        <div className="flex items-center gap-x-2">
                          {skill.pinned && (
                            <PushPin size={14} className="text-yellow-400" />
                          )}
                          <span className="text-theme-text-primary font-medium">
                            {skill.name}
                          </span>
                        </div>
                        <div className="text-xs text-theme-text-secondary truncate max-w-[200px]">
                          {skill.description}
                        </div>
                      </td>
                      <td className="py-3 px-2 text-theme-text-secondary">
                        {skill.category || "—"}
                      </td>
                      <td className="py-3 px-2">
                        <span
                          className={`text-xs px-2 py-1 rounded-full ${
                            STATUS_STYLES[skill.status] || STATUS_STYLES.active
                          }`}
                        >
                          {skill.status}
                        </span>
                      </td>
                      <td className="py-3 px-2 text-theme-text-secondary">
                        {skill.useCount}
                      </td>
                      <td className="py-3 px-2 text-theme-text-secondary">
                        {skill.lastUsedAt
                          ? new Date(skill.lastUsedAt).toLocaleDateString()
                          : "—"}
                      </td>
                      <td className="py-3 px-2 text-theme-text-secondary capitalize">
                        {skill.createdBy}
                      </td>
                      <td className="py-3 px-2">
                        <div className="flex items-center justify-end gap-x-1">
                          <button
                            onClick={() => setViewingSkill(skill)}
                            className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-white"
                            title="View"
                          >
                            <Eye size={16} />
                          </button>
                          <button
                            onClick={() => setEditingSkill(skill)}
                            className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-white"
                            title="Edit"
                          >
                            <Pencil size={16} />
                          </button>
                          <button
                            onClick={() => handleTogglePin(skill)}
                            className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-white"
                            title={skill.pinned ? "Unpin" : "Pin"}
                          >
                            {skill.pinned ? (
                              <PushPinSlash size={16} />
                            ) : (
                              <PushPin size={16} />
                            )}
                          </button>
                          {skill.status !== "archived" ? (
                            <button
                              onClick={() => handleToggleStatus(skill, "archived")}
                              className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-white"
                              title="Archive"
                            >
                              <Archive size={16} />
                            </button>
                          ) : (
                            <button
                              onClick={() => handleToggleStatus(skill, "active")}
                              className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-white"
                              title="Restore"
                            >
                              <ArrowCounterClockwise size={16} />
                            </button>
                          )}
                          <button
                            onClick={() => handleDelete(skill)}
                            className="p-1.5 rounded-md hover:bg-white/10 text-theme-text-secondary hover:text-red-400"
                            title="Delete"
                          >
                            <Trash size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {showCreate && (
        <SkillModal
          workspaceSlug={selectedWsSlug}
          onClose={() => setShowCreate(false)}
          onSaved={() => {
            setShowCreate(false);
            refreshSkills();
          }}
        />
      )}

      {/* Edit Modal */}
      {editingSkill && (
        <SkillModal
          workspaceSlug={selectedWsSlug}
          skill={editingSkill}
          onClose={() => setEditingSkill(null)}
          onSaved={() => {
            setEditingSkill(null);
            refreshSkills();
          }}
        />
      )}

      {/* View Drawer */}
      {viewingSkill && (
        <ViewDrawer
          workspaceSlug={selectedWsSlug}
          skill={viewingSkill}
          onClose={() => setViewingSkill(null)}
        />
      )}
    </div>
  );
}

function SkillModal({ workspaceSlug, skill, onClose, onSaved }) {
  const [name, setName] = useState(skill?.name || "");
  const [description, setDescription] = useState(skill?.description || "");
  const [category, setCategory] = useState(skill?.category || "");
  const [content, setContent] = useState(skill?.content || "");
  const [frontmatter, setFrontmatter] = useState(skill?.frontmatter || "");
  const [saving, setSaving] = useState(false);

  const isEditing = !!skill;

  const handleSave = async () => {
    setSaving(true);
    try {
      if (isEditing) {
        await AgentSkills.update(workspaceSlug, skill.slug, {
          name,
          description,
          category,
          content,
          frontmatter,
        });
      } else {
        const fm = frontmatter || `name: ${name}\ndescription: ${description}\n`;
        await AgentSkills.create(workspaceSlug, {
          name,
          description,
          category,
          content,
          frontmatter: fm,
        });
      }
      showToast(isEditing ? "Skill updated" : "Skill created", "success");
      onSaved();
    } catch (e) {
      showToast(e.message || "Failed to save", "error");
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-theme-bg-secondary rounded-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto p-6 m-4">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-bold text-theme-text-primary">
            {isEditing ? "Edit Skill" : "Create Skill"}
          </h2>
          <button
            onClick={onClose}
            className="text-theme-text-secondary hover:text-white"
          >
            <X size={20} />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="text-sm text-theme-text-secondary">Name</label>
            <input
              className="w-full mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isEditing}
            />
          </div>
          <div>
            <label className="text-sm text-theme-text-secondary">
              Description
            </label>
            <input
              className="w-full mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div>
            <label className="text-sm text-theme-text-secondary">Category</label>
            <input
              className="w-full mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              placeholder="e.g. devops"
            />
          </div>
          <div>
            <label className="text-sm text-theme-text-secondary">
              Frontmatter (YAML)
            </label>
            <textarea
              className="w-full mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary font-mono"
              rows={4}
              value={frontmatter}
              onChange={(e) => setFrontmatter(e.target.value)}
              placeholder={`name: ${name || "skill-name"}\ndescription: ${description || "What this skill does"}`}
            />
          </div>
          <div>
            <label className="text-sm text-theme-text-secondary">
              SKILL.md Content
            </label>
            <textarea
              className="w-full mt-1 bg-theme-bg-sidebar border border-theme-sidebar-border rounded-lg px-3 py-2 text-sm text-theme-text-primary font-mono"
              rows={12}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="# Skill Title\n\n## When to Use\n...\n\n## Procedure\n1. ..."
            />
          </div>
        </div>

        <div className="flex justify-end gap-x-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm text-theme-text-secondary hover:text-white transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !name}
            className="px-4 py-2 rounded-lg text-sm bg-cta-button hover:bg-cta-button-hover text-white disabled:opacity-50 transition-colors"
          >
            {saving ? "Saving..." : isEditing ? "Update" : "Create"}
          </button>
        </div>
      </div>
    </div>
  );
}

function ViewDrawer({ workspaceSlug, skill, onClose }) {
  const [files, setFiles] = useState([]);
  const [activeTab, setActiveTab] = useState("content");
  const [selectedFile, setSelectedFile] = useState(null);

  useEffect(() => {
    async function fetchFiles() {
      try {
        const res = await AgentSkills.listFiles(workspaceSlug, skill.slug);
        setFiles(res.files || []);
      } catch (e) {
        // silently ignore file list errors
      }
    }
    fetchFiles();
  }, [workspaceSlug, skill.slug]);

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-theme-bg-secondary w-full max-w-xl h-full overflow-y-auto p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-bold text-theme-text-primary">
              {skill.name}
            </h2>
            <p className="text-xs text-theme-text-secondary">
              {skill.category && `Category: ${skill.category} · `}
              Status: {skill.status} · Uses: {skill.useCount} · Views:{" "}
              {skill.viewCount}
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-theme-text-secondary hover:text-white"
          >
            <X size={20} />
          </button>
        </div>

        <div className="flex gap-x-2 mb-4 border-b border-white/10 pb-2">
          <button
            onClick={() => {
              setActiveTab("content");
              setSelectedFile(null);
            }}
            className={`text-sm px-3 py-1 rounded-md transition-colors ${
              activeTab === "content"
                ? "bg-white/10 text-white"
                : "text-theme-text-secondary hover:text-white"
            }`}
          >
            SKILL.md
          </button>
          {files.length > 0 && (
            <button
              onClick={() => setActiveTab("files")}
              className={`text-sm px-3 py-1 rounded-md transition-colors ${
                activeTab === "files"
                  ? "bg-white/10 text-white"
                  : "text-theme-text-secondary hover:text-white"
              }`}
            >
              Files ({files.length})
            </button>
          )}
        </div>

        {activeTab === "content" && (
          <div className="space-y-4">
            <div>
              <label className="text-xs text-theme-text-secondary uppercase tracking-wider">
                Frontmatter
              </label>
              <pre className="mt-1 bg-theme-bg-sidebar rounded-lg p-3 text-xs text-theme-text-primary font-mono overflow-x-auto">
                {skill.frontmatter}
              </pre>
            </div>
            <div>
              <label className="text-xs text-theme-text-secondary uppercase tracking-wider">
                Content
              </label>
              <pre className="mt-1 bg-theme-bg-sidebar rounded-lg p-3 text-xs text-theme-text-primary font-mono overflow-x-auto whitespace-pre-wrap">
                {skill.content}
              </pre>
            </div>
          </div>
        )}

        {activeTab === "files" && (
          <div className="space-y-3">
            {files.map((file) => (
              <div key={file.filePath} className="bg-theme-bg-sidebar rounded-lg p-3">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-theme-text-primary font-mono">
                    {file.filePath}
                  </span>
                  <button
                    onClick={() =>
                      setSelectedFile(
                        selectedFile?.filePath === file.filePath ? null : file
                      )
                    }
                    className="text-xs text-theme-text-secondary hover:text-white"
                  >
                    {selectedFile?.filePath === file.filePath
                      ? "Hide"
                      : "View"}
                  </button>
                </div>
                {selectedFile?.filePath === file.filePath && (
                  <pre className="text-xs text-theme-text-primary font-mono overflow-x-auto whitespace-pre-wrap max-h-64 overflow-y-auto">
                    {file.content}
                  </pre>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
