function doPost(e) {
  var data = JSON.parse(e.postData.contents);
  if (data.key !== API_KEY) {
    return jsonResponse({status: "error", error: "unauthorized"});
  }
  var action = data.action;
  try {
    switch (action) {
      case "search":
        return handleSearch(data);
      case "read_thread":
        return handleReadThread(data);
      case "list_drafts":
        return handleListDrafts(data);
      case "get_draft":
        return handleGetDraft(data);
      case "mailbox_stats":
        return handleMailboxStats(data);
      case "create_draft":
        return handleCreateDraft(data);
      case "update_draft":
        return handleUpdateDraft(data);
      case "send_draft":
        return handleSendDraft(data);
      case "send_email":
        return handleSendEmail(data);
      case "reply_to_thread":
        return handleReplyToThread(data);
      case "delete_draft":
        return handleDeleteDraft(data);
      case "move_to_trash":
        return handleMoveToTrash(data);
      default:
        return jsonResponse({status: "error", error: "unknown action: " + action});
    }
  } catch (err) {
    return jsonResponse({status: "error", error: err.toString()});
  }
}

function jsonResponse(obj) {
  return ContentService.createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}

function handleSearch(data) {
  var query = data.query || "";
  var threads = GmailApp.search(query, 0, 20);
  var results = [];
  for (var i = 0; i < threads.length; i++) {
    var msgs = threads[i].getMessages();
    if (msgs.length > 0) {
      var m = msgs[0];
      results.push({
        threadId: threads[i].getId(),
        subject: m.getSubject(),
        from: m.getFrom(),
        date: m.getDate().toISOString()
      });
    }
  }
  return jsonResponse({status: "ok", data: results});
}

function handleReadThread(data) {
  var thread = GmailApp.getThreadById(data.thread_id);
  var msgs = thread.getMessages();
  var results = [];
  for (var i = 0; i < msgs.length; i++) {
    results.push({
      id: msgs[i].getId(),
      subject: msgs[i].getSubject(),
      from: msgs[i].getFrom(),
      to: msgs[i].getTo(),
      body: msgs[i].getPlainBody(),
      date: msgs[i].getDate().toISOString()
    });
  }
  return jsonResponse({status: "ok", data: results});
}

function handleListDrafts(data) {
  var drafts = GmailApp.getDrafts();
  var results = [];
  for (var i = 0; i < Math.min(drafts.length, 20); i++) {
    var m = drafts[i].getMessage();
    results.push({
      draftId: drafts[i].getId(),
      subject: m.getSubject(),
      to: m.getTo()
    });
  }
  return jsonResponse({status: "ok", data: results});
}

function handleGetDraft(data) {
  var draft = GmailApp.getDraftById(data.draft_id);
  var m = draft.getMessage();
  return jsonResponse({status: "ok", data: {
    draftId: draft.getId(),
    subject: m.getSubject(),
    to: m.getTo(),
    body: m.getPlainBody()
  }});
}

function handleMailboxStats(data) {
  return jsonResponse({status: "ok", data: {
    inboxUnread: GmailApp.getInboxUnreadCount(),
    spamUnread: GmailApp.getSpamUnreadCount(),
    totalThreads: GmailApp.search("").length
  }});
}

function handleCreateDraft(data) {
  var draft = GmailApp.createDraft(data.to, data.subject, data.body);
  return jsonResponse({status: "ok", data: {draftId: draft.getId()}});
}

function handleUpdateDraft(data) {
  // GmailApp doesn't support direct draft update; recreate
  var old = GmailApp.getDraftById(data.draft_id);
  old.deleteDraft();
  var draft = GmailApp.createDraft(data.to, data.subject, data.body);
  return jsonResponse({status: "ok", data: {draftId: draft.getId()}});
}

function handleSendDraft(data) {
  var draft = GmailApp.getDraftById(data.draft_id);
  draft.sendDraft();
  return jsonResponse({status: "ok", data: {sent: true}});
}

function handleSendEmail(data) {
  GmailApp.sendEmail(data.to, data.subject, data.body);
  return jsonResponse({status: "ok", data: {sent: true}});
}

function handleReplyToThread(data) {
  var thread = GmailApp.getThreadById(data.thread_id);
  var draft = thread.createDraftReply(data.body);
  return jsonResponse({status: "ok", data: {draftId: draft.getId()}});
}

function handleDeleteDraft(data) {
  var draft = GmailApp.getDraftById(data.draft_id);
  draft.deleteDraft();
  return jsonResponse({status: "ok", data: {deleted: true}});
}

function handleMoveToTrash(data) {
  var thread = GmailApp.getThreadById(data.thread_id);
  thread.moveToTrash();
  return jsonResponse({status: "ok", data: {trashed: true}});
}
