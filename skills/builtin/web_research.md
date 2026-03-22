---
name: web_research
description: Research a topic on the web using browser tools
triggers:
  - "research"
  - "look up"
  - "search for"
  - "find out about"
  - "what is"
  - "who is"
  - "tell me about"
tools:
  - browser_navigate
  - browser_click
  - browser_type
  - browser_extract_dom
  - browser_scroll
  - browser_eval_js
---

# Web Research Skill

When the user asks you to research something or find information online:

1. Navigate to a search engine (prefer Google: https://www.google.com)
2. Type the search query into the search box
3. Press Enter or click the search button
4. Read the search results using `browser_extract_dom`
5. Click on the most relevant result
6. Extract the key information from the page
7. If needed, scroll down or visit additional pages
8. Summarize your findings concisely for the user

Tips:
- Always extract the DOM first to understand the page structure
- Use `browser_scroll` if the content is below the fold
- If a popup appears, dismiss it using `browser_press_key` with 'Escape' or `browser_eval_js`
- Cite your sources (mention the website names)
