function doPost(e) {
  var data = JSON.parse(e.postData.contents);
  if (data.key !== API_KEY) {
    return jsonResponse({status: "error", error: "unauthorized"});
  }
  var action = data.action;
  try {
    switch (action) {
      case "list_calendars":
        return handleListCalendars(data);
      case "get_calendar":
        return handleGetCalendar(data);
      case "get_event":
        return handleGetEvent(data);
      case "get_events_for_day":
        return handleGetEventsForDay(data);
      case "get_events":
        return handleGetEvents(data);
      case "quick_add":
        return handleQuickAdd(data);
      case "create_event":
        return handleCreateEvent(data);
      case "update_event":
        return handleUpdateEvent(data);
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

function handleListCalendars(data) {
  var cals = CalendarApp.getAllCalendars();
  var results = [];
  for (var i = 0; i < cals.length; i++) {
    results.push({id: cals[i].getId(), name: cals[i].getName()});
  }
  return jsonResponse({status: "ok", data: results});
}

function handleGetCalendar(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  return jsonResponse({status: "ok", data: {
    id: cal.getId(), name: cal.getName(), timezone: cal.getTimeZone()
  }});
}

function handleGetEvent(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  var event = cal.getEventById(data.event_id);
  return jsonResponse({status: "ok", data: eventToJson(event)});
}

function handleGetEventsForDay(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  var date = new Date(data.date);
  var events = cal.getEventsForDay(date);
  var results = [];
  for (var i = 0; i < events.length; i++) {
    results.push(eventToJson(events[i]));
  }
  return jsonResponse({status: "ok", data: results});
}

function handleGetEvents(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  var start = new Date(data.start_time);
  var end = new Date(data.end_time);
  var events = cal.getEvents(start, end);
  var results = [];
  for (var i = 0; i < events.length; i++) {
    results.push(eventToJson(events[i]));
  }
  return jsonResponse({status: "ok", data: results});
}

function handleQuickAdd(data) {
  var cal = CalendarApp.getDefaultCalendar();
  var event = cal.createEventFromDescription(data.text);
  return jsonResponse({status: "ok", data: eventToJson(event)});
}

function handleCreateEvent(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  var start = new Date(data.start_time);
  var end = new Date(data.end_time);
  var event = cal.createEvent(data.title, start, end, {description: data.description || ""});
  return jsonResponse({status: "ok", data: eventToJson(event)});
}

function handleUpdateEvent(data) {
  var cal = CalendarApp.getCalendarById(data.calendar_id);
  var event = cal.getEventById(data.event_id);
  event.setTitle(data.title);
  event.setDescription(data.description || "");
  event.setTime(new Date(data.start_time), new Date(data.end_time));
  return jsonResponse({status: "ok", data: eventToJson(event)});
}

function eventToJson(event) {
  return {
    id: event.getId(),
    title: event.getTitle(),
    description: event.getDescription(),
    startTime: event.getStartTime().toISOString(),
    endTime: event.getEndTime().toISOString()
  };
}
