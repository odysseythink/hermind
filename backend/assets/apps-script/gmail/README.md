# Gmail Apps Script Bridge

This Google Apps Script provides the backend for the `gmail-agent` skill in Hermind Go server.

## Deployment Steps

1. Open [script.google.com](https://script.google.com) and create a new project.
2. Replace `Code.gs` with the contents of `Code.gs` from this folder.
3. Replace `appsscript.json` with the contents of `appsscript.json` from this folder.
4. Click **Deploy → New deployment → Web app**.
5. Set:
   - **Execute as**: Me
   - **Who has access**: Anyone with the link
6. Click **Deploy**. You may need to authorize Gmail scopes on first run.
7. Copy the **Deployment ID** from the URL (the long string after `/macros/s/`).
8. In Hermind, set the system setting `gmail_agent_config` to:
   ```json
   {"deploymentId":"YOUR_DEPLOYMENT_ID","apiKey":"YOUR_API_KEY"}
   ```
   The `apiKey` is any secret you choose — it protects the endpoint.

## Quota Limits

Google Apps Script has daily quotas. If exceeded, the bridge returns an error envelope that the agent will surface.
