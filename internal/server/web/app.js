// Tour app state
const state = {
  tours: [],
  currentTour: null,
  currentStep: 0,
  fileCache: {},
};

// File extension to Prism language mapping
const langMap = {
  js: "javascript",
  ts: "typescript",
  tsx: "tsx",
  jsx: "jsx",
  py: "python",
  rb: "ruby",
  rs: "rust",
  go: "go",
  java: "java",
  kt: "kotlin",
  cs: "csharp",
  cpp: "cpp",
  c: "c",
  h: "c",
  hpp: "cpp",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  toml: "toml",
  md: "markdown",
  sql: "sql",
  html: "html",
  css: "css",
  xml: "xml",
  dockerfile: "docker",
  makefile: "makefile",
  mod: "go-module",
  sum: "go-module",
};

function getLang(filename) {
  const ext = filename.split(".").pop().toLowerCase();
  const base = filename.split("/").pop().toLowerCase();
  return langMap[ext] || langMap[base] || "plaintext";
}

// DOM elements
const tourSelect = document.getElementById("tour-select");
const tourTitle = document.getElementById("tour-title");
const stepCounter = document.getElementById("step-counter");
const prevBtn = document.getElementById("prev-btn");
const nextBtn = document.getElementById("next-btn");
const fileHeader = document.getElementById("file-header");
const codeBlock = document.getElementById("code-block");
const codePre = document.getElementById("code-pre");
const stepDescription = document.getElementById("step-description");
const loadingEl = document.getElementById("loading");
const errorEl = document.getElementById("error");

async function init() {
  try {
    // Load session info
    const sessRes = await fetch("/api/session");
    if (sessRes.ok) {
      const sessData = await sessRes.json();
      const sessionNameEl = document.getElementById("session-name");
      if (sessionNameEl) {
        sessionNameEl.textContent = `Session: ${sessData.name}`;
        sessionNameEl.title = sessData.id;
      }
    }

    const res = await fetch("/api/tours");
    if (!res.ok) throw new Error("Failed to load tours");
    state.tours = await res.json();

    if (state.tours.length === 0) {
      showError("No tours found. Create a .tour file in .tours/");
      return;
    }

    // Populate tour selector
    tourSelect.innerHTML = "";
    for (const t of state.tours) {
      const opt = document.createElement("option");
      opt.value = t.name;
      opt.textContent = `${t.title} (${t.steps} steps)`;
      tourSelect.appendChild(opt);
    }

    // Hide selector and show title if only one tour
    if (state.tours.length === 1) {
      tourSelect.style.display = "none";
      tourTitle.hidden = false;
    }

    tourSelect.addEventListener("change", () => loadTour(tourSelect.value));

    await loadTour(state.tours[0].name);
  } catch (err) {
    showError(err.message);
  }
}

async function loadTour(name) {
  try {
    const res = await fetch(`/api/tours/${name}`);
    if (!res.ok) throw new Error(`Failed to load tour: ${name}`);
    state.currentTour = await res.json();
    state.currentTour._name = name;
    state.fileCache = {};

    // Restore step from URL query parameter, default to 0
    const params = new URLSearchParams(window.location.search);
    const savedStep = parseInt(params.get("step") || "0", 10);
    const validStep = Math.max(0, Math.min(savedStep, state.currentTour.steps.length - 1));

    await showStep(validStep);
    hideLoading();
  } catch (err) {
    showError(err.message);
  }
}

async function showStep(index) {
  const tour = state.currentTour;
  if (!tour || index < 0 || index >= tour.steps.length) return;

  state.currentStep = index;
  const step = tour.steps[index];

  // Save step to URL
  const url = new URL(window.location);
  url.searchParams.set("step", index);
  window.history.replaceState(null, "", url);

  // Update header
  if (tourTitle.hidden === false) {
    tourTitle.textContent = tour.title;
  }
  stepCounter.textContent = `Step ${index + 1} of ${tour.steps.length}`;
  prevBtn.disabled = index === 0;
  nextBtn.disabled = index === tour.steps.length - 1;

  // Update file header
  fileHeader.textContent = step.file;

  // Load and display file content
  const content = await loadFile(step.file);
  const lang = getLang(step.file);
  codeBlock.className = `language-${lang}`;
  codeBlock.textContent = content;

  // Set line highlight
  codePre.setAttribute("data-line", step.line);

  // Re-highlight
  Prism.highlightElement(codeBlock);

  // Scroll to highlighted line after rendering
  requestAnimationFrame(() => {
    const lineEl = codePre.querySelector(".line-highlight");
    if (lineEl) {
      lineEl.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  });

  // Render step description as markdown
  stepDescription.innerHTML = marked.parse(step.description);

  // Highlight code blocks in the markdown
  stepDescription.querySelectorAll("pre code").forEach((block) => {
    Prism.highlightElement(block);
  });
}

async function loadFile(path) {
  if (state.fileCache[path]) return state.fileCache[path];

  const res = await fetch(`/api/files/${path}`);
  if (!res.ok) return `// Failed to load ${path}`;

  const text = await res.text();
  state.fileCache[path] = text;
  return text;
}

function showError(msg) {
  hideLoading();
  errorEl.textContent = msg;
  errorEl.hidden = false;
}

function hideLoading() {
  loadingEl.classList.add("hidden");
}

// Navigation
prevBtn.addEventListener("click", () => showStep(state.currentStep - 1));
nextBtn.addEventListener("click", () => showStep(state.currentStep + 1));

document.addEventListener("keydown", (e) => {
  // Don't capture if user is typing in an input
  if (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA") return;

  if (e.key === "ArrowLeft" || e.key === "h") {
    showStep(state.currentStep - 1);
  } else if (e.key === "ArrowRight" || e.key === "l") {
    showStep(state.currentStep + 1);
  }
});

// Chat
const chatMessages = document.getElementById("chat-messages");
const chatInput = document.getElementById("chat-input");
const chatSend = document.getElementById("chat-send");
const chatReset = document.getElementById("chat-reset");

let ws = null;
let currentAssistantEl = null;

function resetChat() {
  // Clear chat messages
  chatMessages.innerHTML = "";
  currentAssistantEl = null;
  chatInput.disabled = false;
  chatInput.focus();

  // Close existing WebSocket
  if (ws) {
    ws.close();
    ws = null;
  }

  // Reconnect (will start a fresh session on the server)
  connectChat();
}

function connectChat() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(`${proto}//${location.host}/ws/chat`);

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);

    if (msg.type === "token") {
      if (!currentAssistantEl) {
        currentAssistantEl = addChatMessage("assistant", "");
      }
      const textEl = currentAssistantEl.querySelector(".chat-msg-text");
      // Store raw text in data attribute, append visible content
      textEl.dataset.rawContent = (textEl.dataset.rawContent || "") + msg.content;
      textEl.textContent = textEl.dataset.rawContent;
      chatMessages.scrollTop = chatMessages.scrollHeight;
    } else if (msg.type === "done") {
      // Render markdown when message is complete
      if (currentAssistantEl) {
        const textEl = currentAssistantEl.querySelector(".chat-msg-text");
        const rawContent = textEl.dataset.rawContent || "";
        textEl.innerHTML = marked.parse(rawContent);
        // Highlight code blocks in chat
        textEl.querySelectorAll("pre code").forEach((block) => {
          Prism.highlightElement(block);
        });
      }
      currentAssistantEl = null;
      chatInput.disabled = false;
      chatInput.focus();
    } else if (msg.type === "error") {
      addChatMessage("error", msg.message);
      currentAssistantEl = null;
      chatInput.disabled = false;
      chatInput.focus();
    }
  };

  ws.onclose = () => {
    // Reconnect after a short delay
    setTimeout(connectChat, 2000);
  };
}

function addChatMessage(role, text) {
  const el = document.createElement("div");
  el.className = `chat-msg chat-msg-${role}`;

  const label = role === "error" ? "Error" : role === "assistant" ? "Claude" : "You";
  el.innerHTML = `<span class="chat-msg-label">${label}:</span><span class="chat-msg-text"></span>`;
  el.querySelector(".chat-msg-text").textContent = text;

  chatMessages.appendChild(el);
  chatMessages.scrollTop = chatMessages.scrollHeight;
  return el;
}

function sendChat() {
  const text = chatInput.value.trim();
  if (!text || !ws || ws.readyState !== WebSocket.OPEN) return;

  const step = state.currentTour?.steps[state.currentStep];

  addChatMessage("user", text);
  chatInput.value = "";
  chatInput.disabled = true;

  ws.send(JSON.stringify({
    type: "chat",
    message: text,
    context: {
      tour: state.currentTour?._name || "",
      step: state.currentStep,
      file: step?.file || "",
      line: step?.line || 0,
    },
  }));
}

chatSend.addEventListener("click", sendChat);
chatInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") sendChat();
});
chatReset.addEventListener("click", resetChat);

// Chat panel resizing
const chatPanel = document.getElementById("chat-panel");
const resizeHandle = document.getElementById("chat-resize-handle");
let isResizing = false;

// Set initial chat panel height: use saved value or default to 50% of viewport
function initializeChatHeight() {
  const savedHeight = localStorage.getItem("chatPanelHeight");
  if (savedHeight) {
    chatPanel.style.height = savedHeight + "px";
  } else {
    // Default to 50% of viewport height
    const defaultHeight = window.innerHeight * 0.5;
    chatPanel.style.height = defaultHeight + "px";
  }
}

initializeChatHeight();

// Re-initialize on window resize to keep proportions
window.addEventListener("resize", () => {
  const savedHeight = localStorage.getItem("chatPanelHeight");
  if (!savedHeight) {
    // Only reset to 50% if no custom height was saved
    const defaultHeight = window.innerHeight * 0.5;
    chatPanel.style.height = defaultHeight + "px";
  }
});

resizeHandle.addEventListener("mousedown", (e) => {
  isResizing = true;
  const startY = e.clientY;
  const startHeight = chatPanel.offsetHeight;

  const onMouseMove = (moveEvent) => {
    if (!isResizing) return;
    const delta = moveEvent.clientY - startY;
    const newHeight = Math.max(100, startHeight - delta);
    chatPanel.style.height = newHeight + "px";
    localStorage.setItem("chatPanelHeight", newHeight);
  };

  const onMouseUp = () => {
    isResizing = false;
    document.removeEventListener("mousemove", onMouseMove);
    document.removeEventListener("mouseup", onMouseUp);
  };

  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", onMouseUp);
});

// Strip raw HTML from markdown to prevent XSS
marked.use({ renderer: { html() { return ""; } } });

// Start
connectChat();
init();
