import { API_BASE } from "@/utils/constants";
import { baseHeaders } from "@/utils/request";

const AgentSkills = {
  list: async function (workspaceSlug, includeArchived = false) {
    const url = new URL(`${API_BASE}/workspace/${workspaceSlug}/agent-skills`, window.location.origin);
    if (includeArchived) url.searchParams.set("include_archived", "true");
    const res = await fetch(url.toString(), {
      headers: baseHeaders(),
    }).then((r) => r.json());
    return res;
  },

  get: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      { headers: baseHeaders() }
    ).then((r) => r.json());
    return res;
  },

  create: async function (workspaceSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills`,
      {
        method: "POST",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },

  update: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "PUT",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },

  patch: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "PATCH",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },

  delete: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "DELETE",
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },

  listFiles: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files`,
      { headers: baseHeaders() }
    ).then((r) => r.json());
    return res;
  },

  writeFile: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files`,
      {
        method: "POST",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },

  deleteFile: async function (workspaceSlug, skillSlug, filePath) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files/${encodeURIComponent(filePath)}`,
      {
        method: "DELETE",
        headers: baseHeaders(),
      }
    ).then((r) => r.json());
    return res;
  },
};

export default AgentSkills;
