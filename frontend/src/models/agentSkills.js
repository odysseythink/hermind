import { API_BASE } from "@/utils/constants";
import { baseHeaders } from "@/utils/request";

async function handleResponse(res) {
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status}: ${text || res.statusText}`);
  }
  return res.json();
}

const AgentSkills = {
  list: async function (workspaceSlug, includeArchived = false) {
    const url = new URL(`${API_BASE}/workspace/${workspaceSlug}/agent-skills`, window.location.origin);
    if (includeArchived) url.searchParams.set("include_archived", "true");
    const res = await fetch(url.toString(), {
      headers: baseHeaders(),
    });
    return handleResponse(res);
  },

  get: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      { headers: baseHeaders() }
    );
    return handleResponse(res);
  },

  create: async function (workspaceSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills`,
      {
        method: "POST",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },

  update: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "PUT",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },

  patch: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "PATCH",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },

  delete: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}`,
      {
        method: "DELETE",
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },

  listFiles: async function (workspaceSlug, skillSlug) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files`,
      { headers: baseHeaders() }
    );
    return handleResponse(res);
  },

  writeFile: async function (workspaceSlug, skillSlug, data) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files`,
      {
        method: "POST",
        body: JSON.stringify(data),
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },

  deleteFile: async function (workspaceSlug, skillSlug, filePath) {
    const res = await fetch(
      `${API_BASE}/workspace/${workspaceSlug}/agent-skills/${skillSlug}/files/${encodeURIComponent(filePath)}`,
      {
        method: "DELETE",
        headers: baseHeaders(),
      }
    );
    return handleResponse(res);
  },
};

export default AgentSkills;
