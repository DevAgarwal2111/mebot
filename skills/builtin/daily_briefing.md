---
name: daily_briefing
description: Gives the user a morning briefing with calendar events and unread emails
triggers:
  - "briefing"
  - "morning update"
  - "daily summary"
  - "what's on today"
  - "good morning"
  - "start my day"
tools:
  - google_calendar_list_events
  - google_gmail_list_unread
  - get_current_time
cron: "0 9 * * *"
---

# Daily Briefing Skill

When this skill is activated, give the user a comprehensive morning briefing:

1. First, get the current time using `get_current_time` with timezone "Asia/Kolkata"
2. Fetch today's calendar events using `google_calendar_list_events`
3. Fetch unread emails using `google_gmail_list_unread`
4. Present everything in a clean, organized summary:
   - Greet the user with the current date/time
   - List today's events in chronological order
   - Summarize the most important unread emails (sender, subject, preview)
   - If there are no events, mention that the day looks clear
   - End with a motivational note or a fun fact
