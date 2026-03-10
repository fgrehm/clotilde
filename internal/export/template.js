(function () {
  "use strict";

  // --- Data loading ---
  var dataEl = document.getElementById("session-data");
  if (!dataEl) return;

  var b64 = dataEl.textContent.trim();
  var binaryStr = atob(b64);
  var bytes = new Uint8Array(binaryStr.length);
  for (var i = 0; i < binaryStr.length; i++) {
    bytes[i] = binaryStr.charCodeAt(i);
  }
  var raw = new TextDecoder("utf-8").decode(bytes);
  var data = JSON.parse(raw);
  var sessionName = data.sessionName || "unnamed";
  var entries = data.entries || [];

  // --- Configure marked ---
  if (typeof marked !== "undefined") {
    var renderer = new marked.Renderer();
    // Escape raw HTML in transcript content to prevent XSS
    renderer.html = function (text) {
      var src = typeof text === "object" ? text.raw || text.text || "" : text;
      return escapeHtml(src);
    };
    // Only allow safe URL schemes in links and images
    renderer.link = function (token) {
      var href = (typeof token === "object" ? token.href : token) || "";
      var text = (typeof token === "object" ? token.text : arguments[1]) || href;
      if (!isSafeUrl(href)) return escapeHtml(text);
      return '<a href="' + escapeHtml(href) + '">' + escapeHtml(text) + "</a>";
    };
    renderer.image = function (token) {
      var src = (typeof token === "object" ? token.href : token) || "";
      var alt = (typeof token === "object" ? token.text : arguments[1]) || "";
      if (!isSafeUrl(src)) return escapeHtml(alt);
      return '<img src="' + escapeHtml(src) + '" alt="' + escapeHtml(alt) + '">';
    };
    marked.setOptions({
      breaks: true,
      gfm: true,
      renderer: renderer,
    });
  }

  // --- Tool result pairing ---
  // Build Map<tool_use_id, tool_result_content>
  var toolResults = {};
  entries.forEach(function (entry) {
    if (entry.type !== "user") return;
    var msg = entry.message;
    if (!msg || !Array.isArray(msg.content)) return;
    msg.content.forEach(function (block) {
      if (block.type === "tool_result" && block.tool_use_id) {
        toolResults[block.tool_use_id] = block;
      }
    });
  });

  // --- Stats computation ---
  var totalTokensIn = 0;
  var totalTokensOut = 0;
  var turnCount = 0;
  var lastModel = "";
  var firstTimestamp = null;
  var lastTimestamp = null;

  entries.forEach(function (entry) {
    var ts = entry.timestamp ? new Date(entry.timestamp) : null;
    if (ts && !isNaN(ts)) {
      if (!firstTimestamp || ts < firstTimestamp) firstTimestamp = ts;
      if (!lastTimestamp || ts > lastTimestamp) lastTimestamp = ts;
    }

    if (entry.type === "user") {
      var msg = entry.message;
      if (msg && typeof msg.content === "string") {
        turnCount++;
      }
    }

    if (entry.type === "assistant" && entry.message) {
      if (entry.message.model) lastModel = entry.message.model;
      var usage = entry.message.usage;
      if (usage) {
        totalTokensIn += (usage.input_tokens || 0) +
          (usage.cache_creation_input_tokens || 0) +
          (usage.cache_read_input_tokens || 0);
        totalTokensOut += usage.output_tokens || 0;
      }
    }
  });

  // --- Render header ---
  var headerEl = document.getElementById("header-container");
  var titleEl = document.createElement("div");
  titleEl.className = "header-title";
  titleEl.textContent = sessionName;
  headerEl.appendChild(titleEl);

  var statsEl = document.createElement("div");
  statsEl.className = "header-stats";

  if (lastModel) {
    statsEl.appendChild(statSpan("Model: " + formatModel(lastModel)));
  }
  statsEl.appendChild(statSpan("Turns: " + turnCount));

  var totalTokens = totalTokensIn + totalTokensOut;
  if (totalTokens > 0) {
    statsEl.appendChild(statSpan("Tokens: " + formatNumber(totalTokens) +
      " (" + formatNumber(totalTokensIn) + " in, " + formatNumber(totalTokensOut) + " out)"));
  }

  if (firstTimestamp) {
    statsEl.appendChild(statSpan("Started: " + formatDate(firstTimestamp)));
  }
  if (firstTimestamp && lastTimestamp && lastTimestamp > firstTimestamp) {
    var durationMs = lastTimestamp - firstTimestamp;
    statsEl.appendChild(statSpan("Duration: " + formatDuration(durationMs)));
  }
  statsEl.appendChild(statSpan("Messages: " + entries.length));
  headerEl.appendChild(statsEl);

  // --- Render messages ---
  var messagesEl = document.getElementById("messages");

  entries.forEach(function (entry) {
    if (entry.type === "user") {
      renderUserMessage(messagesEl, entry);
    } else if (entry.type === "assistant") {
      renderAssistantMessage(messagesEl, entry);
    }
  });

  // --- Syntax highlighting ---
  if (typeof hljs !== "undefined") {
    document.querySelectorAll("pre code[class*='language-']").forEach(function (el) {
      hljs.highlightElement(el);
    });
  }

  // --- Keyboard shortcuts ---
  document.addEventListener("keydown", function (e) {
    // Ctrl+T: toggle all thinking blocks
    if (e.ctrlKey && e.key === "t") {
      e.preventDefault();
      var blocks = document.querySelectorAll(".thinking-content");
      var anyVisible = false;
      blocks.forEach(function (b) {
        if (b.classList.contains("visible")) anyVisible = true;
      });
      blocks.forEach(function (b) {
        b.classList.toggle("visible", !anyVisible);
      });
    }
    // Ctrl+O: toggle all tool outputs
    if (e.ctrlKey && e.key === "o") {
      e.preventDefault();
      var outputs = document.querySelectorAll(".tool-output-content");
      var anyExpanded = false;
      outputs.forEach(function (o) {
        if (o.classList.contains("expanded")) anyExpanded = true;
      });
      outputs.forEach(function (o) {
        o.classList.toggle("expanded", !anyExpanded);
      });
    }
  });

  // ==================
  // Render functions
  // ==================

  function renderUserMessage(container, entry) {
    var msg = entry.message;
    if (!msg) return;

    // Skip tool result messages (array content)
    if (Array.isArray(msg.content)) return;

    // Only render string content (human prompts)
    if (typeof msg.content !== "string") return;

    var card = createMessageCard("user", entry.timestamp);
    var body = card.querySelector(".message-body");
    body.innerHTML = renderMarkdown(msg.content);
    container.appendChild(card);
  }

  function renderAssistantMessage(container, entry) {
    var msg = entry.message;
    if (!msg || !Array.isArray(msg.content)) return;

    var card = createMessageCard("assistant", entry.timestamp);
    var body = card.querySelector(".message-body");

    msg.content.forEach(function (block) {
      switch (block.type) {
        case "text":
          var textDiv = document.createElement("div");
          textDiv.innerHTML = renderMarkdown(block.text || "");
          body.appendChild(textDiv);
          break;

        case "thinking":
          renderThinkingBlock(body, block);
          break;

        case "tool_use":
          renderToolCall(body, block);
          break;
      }
    });

    container.appendChild(card);
  }

  function renderThinkingBlock(container, block) {
    var div = document.createElement("div");
    div.className = "thinking-block";

    var toggle = document.createElement("div");
    toggle.className = "thinking-toggle";
    toggle.textContent = "Thinking...";
    div.appendChild(toggle);

    var content = document.createElement("div");
    content.className = "thinking-content";
    content.textContent = block.thinking || "";
    div.appendChild(content);

    div.addEventListener("click", function () {
      content.classList.toggle("visible");
    });

    container.appendChild(div);
  }

  function renderToolCall(container, block) {
    var result = toolResults[block.id];
    var isError = result && result.is_error;

    var div = document.createElement("div");
    div.className = "tool-call" + (isError ? " tool-error" : "");

    // Summary line
    var summary = document.createElement("div");
    summary.className = "tool-summary";
    renderToolSummary(summary, block);
    div.appendChild(summary);

    // Tool output (result)
    if (result) {
      renderToolOutput(div, block, result);
    }

    container.appendChild(div);
  }

  function renderToolSummary(container, block) {
    var name = block.name || "Unknown";
    var input = block.input || {};

    var nameEl = document.createElement("span");
    nameEl.className = "tool-name";

    var detailEl = document.createElement("span");
    detailEl.className = "tool-detail";

    switch (name) {
      case "Bash":
        nameEl.textContent = "$";
        var cmdEl = document.createElement("span");
        cmdEl.className = "tool-command";
        cmdEl.textContent = input.command || "";
        container.appendChild(nameEl);
        container.appendChild(cmdEl);
        if (input.description) {
          var descEl = document.createElement("span");
          descEl.className = "tool-detail";
          descEl.textContent = "(" + input.description + ")";
          container.appendChild(descEl);
        }
        return;

      case "Read":
        nameEl.textContent = "read";
        var readPath = input.file_path || "";
        if (input.offset || input.limit) {
          readPath += ":" + (input.offset || 1) + "-" + ((input.offset || 1) + (input.limit || 0));
        }
        detailEl.textContent = readPath;
        break;

      case "Write":
        nameEl.textContent = "write";
        var lines = (input.content || "").split("\n").length;
        detailEl.textContent = (input.file_path || "") + " (" + lines + " lines)";
        break;

      case "Edit":
      case "MultiEdit":
        nameEl.textContent = name === "MultiEdit" ? "multi-edit" : "edit";
        detailEl.textContent = input.file_path || "";
        break;

      case "Grep":
        nameEl.textContent = "grep";
        detailEl.textContent = (input.pattern || "") + (input.path ? " " + input.path : "");
        break;

      case "Glob":
        nameEl.textContent = "glob";
        detailEl.textContent = (input.pattern || "") + (input.path ? " in " + input.path : "");
        break;

      case "LS":
        nameEl.textContent = "ls";
        detailEl.textContent = input.path || ".";
        break;

      case "Task":
      case "Agent":
        nameEl.textContent = "agent";
        detailEl.textContent = input.description || input.prompt || "";
        break;

      case "TodoRead":
        nameEl.textContent = "todo-read";
        break;

      case "TodoWrite":
        nameEl.textContent = "todo-write";
        break;

      case "WebFetch":
        nameEl.textContent = "web-fetch";
        detailEl.textContent = input.url || "";
        break;

      case "WebSearch":
        nameEl.textContent = "web-search";
        detailEl.textContent = input.query || "";
        break;

      case "ToolSearch":
        nameEl.textContent = "tool-search";
        detailEl.textContent = input.query || "";
        break;

      case "Skill":
        nameEl.textContent = "skill";
        detailEl.textContent = JSON.stringify(input);
        break;

      default:
        nameEl.textContent = name;
        detailEl.textContent = truncate(JSON.stringify(input), 200);
        break;
    }

    container.appendChild(nameEl);
    container.appendChild(detailEl);
  }

  function renderToolOutput(container, block, result) {
    var outputDiv = document.createElement("div");
    outputDiv.className = "tool-output";

    var content = extractResultContent(result);
    if (content == null) return;

    var lines = content.split("\n");
    var previewLines = 5;
    var hasMore = lines.length > previewLines;

    var contentEl = document.createElement("div");
    contentEl.className = "tool-output-content" + (hasMore ? " has-more" : "");
    if (result.is_error) contentEl.classList.add("tool-error-output");

    // For Read/Write with file path, try syntax highlighting
    var toolName = block.name || "";
    if ((toolName === "Read" || toolName === "Write") && block.input) {
      var filePath = block.input.file_path || "";
      var lang = guessLanguage(filePath);
      if (lang && typeof hljs !== "undefined") {
        var pre = document.createElement("pre");
        var code = document.createElement("code");
        code.className = "language-" + lang;
        code.textContent = content;
        pre.appendChild(code);
        contentEl.appendChild(pre);
      } else {
        contentEl.textContent = content;
      }
    } else if (toolName === "Edit" || toolName === "MultiEdit") {
      renderEditDiff(contentEl, block, content);
    } else {
      contentEl.textContent = content;
    }

    outputDiv.appendChild(contentEl);

    if (hasMore) {
      var toggleEl = document.createElement("div");
      toggleEl.className = "tool-output-toggle";
      toggleEl.textContent = "Show all (" + lines.length + " lines)";
      toggleEl.addEventListener("click", function () {
        var expanded = contentEl.classList.toggle("expanded");
        toggleEl.textContent = expanded
          ? "Collapse"
          : "Show all (" + lines.length + " lines)";
      });
      outputDiv.appendChild(toggleEl);
    }

    container.appendChild(outputDiv);
  }

  function renderEditDiff(container, block, resultContent) {
    var input = block.input || {};

    if (input.old_string && input.new_string) {
      var pre = document.createElement("pre");
      var code = document.createElement("code");

      var oldLines = input.old_string.split("\n");
      var newLines = input.new_string.split("\n");

      var diffHtml = "";
      oldLines.forEach(function (line) {
        diffHtml += '<span class="diff-line-remove">- ' + escapeHtml(line) + "</span>\n";
      });
      newLines.forEach(function (line) {
        diffHtml += '<span class="diff-line-add">+ ' + escapeHtml(line) + "</span>\n";
      });

      code.innerHTML = diffHtml;
      pre.appendChild(code);
      container.appendChild(pre);
    } else {
      container.textContent = resultContent;
    }
  }

  // ==================
  // Helpers
  // ==================

  function createMessageCard(role, timestamp) {
    var card = document.createElement("div");
    card.className = "message message-" + role;

    var header = document.createElement("div");
    header.className = "message-header";

    var roleEl = document.createElement("span");
    roleEl.textContent = role === "user" ? "You" : "Assistant";
    header.appendChild(roleEl);

    if (timestamp) {
      var tsEl = document.createElement("span");
      tsEl.textContent = formatTime(new Date(timestamp));
      header.appendChild(tsEl);
    }

    card.appendChild(header);

    var body = document.createElement("div");
    body.className = "message-body";
    card.appendChild(body);

    return card;
  }

  function renderMarkdown(text) {
    if (typeof marked !== "undefined") {
      return marked.parse(text);
    }
    return escapeHtml(text).replace(/\n/g, "<br>");
  }

  function extractResultContent(result) {
    if (!result) return null;
    var c = result.content;
    if (typeof c === "string") return c;
    if (Array.isArray(c)) {
      // Array of objects (e.g. tool_reference) - stringify
      return c.map(function (item) {
        if (typeof item === "string") return item;
        if (item.text) return item.text;
        return JSON.stringify(item);
      }).join("\n");
    }
    return null;
  }

  function statSpan(text) {
    var el = document.createElement("span");
    el.textContent = text;
    return el;
  }

  function formatModel(model) {
    // "claude-sonnet-4-5-20250929" -> "Sonnet 4.5"
    var match = model.match(/claude-(?:\d+-)*(\w+)-(\d+)-(\d+)-/);
    if (match) {
      var family = match[1].charAt(0).toUpperCase() + match[1].slice(1);
      return family + " " + match[2] + "." + match[3];
    }
    // "claude-sonnet-4-20250514" -> "Sonnet 4"
    match = model.match(/claude-(?:\d+-)*(\w+)-(\d+)-\d{8}/);
    if (match) {
      var fam = match[1].charAt(0).toUpperCase() + match[1].slice(1);
      return fam + " " + match[2];
    }
    return model;
  }

  function formatNumber(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + "M";
    if (n >= 1000) return (n / 1000).toFixed(1) + "k";
    return String(n);
  }

  function formatDate(d) {
    var months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun",
      "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    return months[d.getMonth()] + " " + d.getDate() + ", " + d.getFullYear() +
      " " + pad2(d.getHours()) + ":" + pad2(d.getMinutes());
  }

  function formatTime(d) {
    if (isNaN(d)) return "";
    return pad2(d.getHours()) + ":" + pad2(d.getMinutes()) + ":" + pad2(d.getSeconds());
  }

  function formatDuration(ms) {
    var s = Math.floor(ms / 1000);
    var m = Math.floor(s / 60);
    var h = Math.floor(m / 60);
    s = s % 60;
    m = m % 60;
    if (h > 0) return h + "h " + m + "m";
    if (m > 0) return m + "m " + s + "s";
    return s + "s";
  }

  function pad2(n) {
    return n < 10 ? "0" + n : String(n);
  }

  function truncate(str, len) {
    if (str.length <= len) return str;
    return str.substring(0, len) + "...";
  }

  function escapeHtml(text) {
    var div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  function isSafeUrl(url) {
    if (!url) return false;
    var lower = url.trim().toLowerCase();
    return lower.startsWith("http://") ||
      lower.startsWith("https://") ||
      lower.startsWith("mailto:") ||
      lower.startsWith("#") ||
      lower.startsWith("/");
  }

  function guessLanguage(filePath) {
    if (!filePath) return null;
    var ext = filePath.split(".").pop().toLowerCase();
    var map = {
      go: "go",
      js: "javascript",
      jsx: "javascript",
      ts: "typescript",
      tsx: "typescript",
      py: "python",
      rs: "rust",
      rb: "ruby",
      java: "java",
      c: "c",
      h: "c",
      cpp: "cpp",
      cc: "cpp",
      hpp: "cpp",
      cs: "csharp",
      css: "css",
      html: "html",
      htm: "html",
      json: "json",
      yaml: "yaml",
      yml: "yaml",
      xml: "xml",
      sql: "sql",
      sh: "bash",
      bash: "bash",
      zsh: "bash",
      md: "markdown",
      toml: "toml",
      makefile: "makefile",
    };
    // Handle Dockerfile, Makefile by filename
    var filename = filePath.split("/").pop().toLowerCase();
    if (filename === "dockerfile") return "dockerfile";
    if (filename === "makefile" || filename === "gnumakefile") return "makefile";
    return map[ext] || null;
  }
})();
