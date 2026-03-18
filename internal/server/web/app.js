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

    // Hide selector if only one tour
    if (state.tours.length === 1) {
      tourSelect.style.display = "none";
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
    state.currentStep = 0;
    state.fileCache = {};
    await showStep(0);
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

  // Update nav
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

// Start
init();
