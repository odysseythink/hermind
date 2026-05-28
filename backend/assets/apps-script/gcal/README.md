# Google Calendar Apps Script Bridge

This Google Apps Script provides the backend for the `google-calendar-agent` skill in AnythingLLM Go server.

## Deployment Steps

1. Open [script.google.com](https://script.google.com) and create a new project.
2. Replace `Code.gs` with the contents of `Code.gs` from this folder.
3. Replace `appsscript.json` with the contents of `appsscript.json` from this folder.
4. Click **Deploy → New deployment → Web app**.
5. Set:
   - **Execute as**: Me
   - **Who has access**: Anyone with the link
6. Click **Deploy**. You may need to authorize Calendar scopes on first run.
7. Copy the **Deployment ID** from the URL.
8. In AnythingLLM, set the system setting `google_calendar_agent_config` to:
   ```json
   {"deploymentId":"YOUR_DEPLOYMENT_ID","apiKey":"YOUR_API_KEY"}
   ```
