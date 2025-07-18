import { api } from "./api.js";
import { getCurrentUser, redirectToLogin } from "./auth.js";

document.addEventListener("DOMContentLoaded", async () => {
  const user = await getCurrentUser();
  if (!user) {
    redirectToLogin();
    return;
  }

  const urlParams = new URLSearchParams(window.location.search);
  const projectId = urlParams.get("projectId");
  const projectName = urlParams.get("projectName");

  if (!projectId || !projectName) {
    window.location.href = "/projects.html";
    return;
  }

  // DOM Elements
  const projectNameSpan = document.getElementById("project-name");
  const searchForm = document.getElementById("search-form");
  const logsTbody = document.getElementById("logs-tbody");
  const noLogsMessage = document.getElementById("no-logs");
  const logDetailsModal = document.getElementById("log-details-modal");
  const logDetailsContent = document.getElementById("log-details-content");
  const prevLogBtn = document.getElementById("prev-log-btn");
  const nextLogBtn = document.getElementById("next-log-btn");
  const closeBtn = document.querySelector(".close-btn");

  // State
  let logs = [];
  let individualLogs = [];
  let currentLogIndex = -1;
  let currentSearchParams = null;

  // --- Main Application Functions ---

  const showModal = () => {
    logDetailsModal.style.display = "block";
  };

  const hideModal = () => {
    logDetailsModal.style.display = "none";
  };

  const renderLogs = () => {
    logsTbody.innerHTML = "";
    if (logs.length === 0) {
      noLogsMessage.style.display = "block";
      logsTbody.parentElement.style.display = "none";
    } else {
      noLogsMessage.style.display = "none";
      logsTbody.parentElement.style.display = "table";
      logs.forEach((log) => {
        const row = document.createElement("tr");
        row.innerHTML = `
          <td>${log.event_name}</td>
          <td>${log.total_count}</td>
          <td>${new Date(log.last_seen).toLocaleString()}</td>
          <td><button class="view-details-btn" data-event-name="${
            log.event_name
          }">View Logs</button></td>
        `;
        logsTbody.appendChild(row);
      });
    }
  };

  const fetchAndRenderLogs = async (params) => {
    try {
      logs = await api.getAggregatedLogs(projectId, params);
      renderLogs();
    } catch (error) {
      console.error("Error fetching logs:", error);
      noLogsMessage.textContent = "Failed to load logs.";
      noLogsMessage.style.display = "block";
    }
  };

  const fetchIndividualLogs = async (eventName) => {
    try {
      individualLogs = await api.getLogs(projectId, { event_name: eventName });
      if (individualLogs.length > 0) {
        currentLogIndex = 0;
        showLogDetails(currentLogIndex);
        showModal();
      } else {
        alert(`No individual logs found for event: ${eventName}`);
      }
    } catch (error) {
      console.error("Error fetching individual logs:", error);
      alert("Failed to load individual logs.");
    }
  };

  // --- Log Details Rendering ---

  // Helper function to safely escape HTML
  function escapeHTML(str) {
    if (typeof str !== 'string') return '';
    return str.replace(/[&<>"']/g, (m) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[m]));
  }

  // Helper function to render a key-value object as a grid
  function renderKeyValueGrid(obj) {
    if (!obj || typeof obj !== "object" || Object.keys(obj).length === 0) {
      return '<div style="color:#888; font-style:italic; padding: 0.5rem;">(No data)</div>';
    }
    return (
      '<div class="log-details-grid">' +
      Object.entries(obj)
        .map(
          ([k, v]) =>
            `<div class="log-details-key">${escapeHTML(k)}:</div>
             <div class="log-details-value">${escapeHTML(String(v))}</div>`
        )
        .join("") +
      "</div>"
    );
  }

  // The main function to render the pretty modal content
  function showLogDetails(index) {
    currentLogIndex = index;
    const log = individualLogs[currentLogIndex];

    const {
      event_name,
      timestamp,
      id,
      project_id,
      searchable_keys,
      payload,
      ...rest // Capture any other fields
    } = log;

    // A primary info card
    const primaryHTML = `
      <div class="log-details-card">
        <div class="log-details-title">
          <span class="log-icon">📄</span>
          ${escapeHTML(event_name || "(no event name)")}
        </div>
        <span class="log-details-timestamp">
          ${timestamp ? new Date(timestamp).toLocaleString() : "No timestamp"}
        </span>
        <div class="log-details-grid">
          <div class="log-details-key">Log ID:</div>
          <div class="log-details-value">${escapeHTML(id || "N/A")}</div>
          <div class="log-details-key">Project ID:</div>
          <div class="log-details-value">${escapeHTML(project_id || "N/A")}</div>
        </div>
      </div>
    `;

    // A section for any other top-level fields
    const otherFieldsHTML = Object.keys(rest).length > 0
      ? `<div class="log-details-section"><h4>Other Metadata</h4>${renderKeyValueGrid(rest)}</div>`
      : "";

    // Compose final modal content
    logDetailsContent.innerHTML = `
      <div class="log-details-main">
        ${primaryHTML}
      </div>
      <div class="log-details-section">
        <h4>Searchable Keys</h4>
        ${renderKeyValueGrid(searchable_keys)}
      </div>
      <div class="log-details-section">
        <h4>Payload</h4>
        ${renderKeyValueGrid(payload)}
      </div>
      ${otherFieldsHTML}
    `;

    updateNavButtons();
  }

  const updateNavButtons = () => {
    prevLogBtn.disabled = currentLogIndex <= 0;
    nextLogBtn.disabled = currentLogIndex >= individualLogs.length - 1;
  };

  // --- Event Handlers ---

  const handleSearch = async (e) => {
    e.preventDefault();
    // ... search logic remains the same
  };

  const handleViewDetails = (e) => {
    if (e.target.classList.contains("view-details-btn")) {
      const eventName = e.target.dataset.eventName;
      fetchIndividualLogs(eventName);
    }
  };

  const handlePrevLog = () => {
    if (currentLogIndex > 0) {
      showLogDetails(currentLogIndex - 1);
    }
  };

  const handleNextLog = () => {
    if (currentLogIndex < individualLogs.length - 1) {
      showLogDetails(currentLogIndex + 1);
    }
  };

  // --- Initial Setup ---
  projectNameSpan.textContent = projectName;
  searchForm.addEventListener("submit", handleSearch);
  logsTbody.addEventListener("click", handleViewDetails);
  prevLogBtn.addEventListener("click", handlePrevLog);
  nextLogBtn.addEventListener("click", handleNextLog);
  closeBtn.addEventListener("click", hideModal);
  window.addEventListener("click", (e) => {
    if (e.target == logDetailsModal) {
      hideModal();
    }
  });

  // Initial Load
  fetchAndRenderLogs({});
});