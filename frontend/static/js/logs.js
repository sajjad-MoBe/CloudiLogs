import { api } from "./api.js";
import { getCurrentUser, redirectToLogin } from "./auth.js";

document.addEventListener("DOMContentLoaded", async () => {
  // 1. Authentication and Initial Setup
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

  // 2. DOM Elements
  const projectNameSpan = document.getElementById("project-name");
  const searchForm = document.getElementById("search-form");
  const logsTbody = document.getElementById("logs-tbody");
  const noLogsMessage = document.getElementById("no-logs");
  const logDetailsModal = document.getElementById("log-details-modal");
  const logDetailsContent = document.getElementById("log-details-content");
  const prevLogBtn = document.getElementById("prev-log-btn");
  const nextLogBtn = document.getElementById("next-log-btn");
  const closeBtn = document.querySelector(".close-btn");

  // 3. State Management
  let logs = [];
  let individualLogs = [];
  let currentLogIndex = -1;
  let currentSearchParams = null;

  // 4. Core Functions

  /**
   * Fetches aggregated logs from the API and renders them in the main table.
   * @param {object} params - The search parameters for the API call.
   */
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

  /**
   * Renders the aggregated logs into the table body.
   */
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
          <td><button class="view-details-btn" data-event-name="${log.event_name}">View Logs</button></td>
        `;
        logsTbody.appendChild(row);
      });
    }
  };

  /**
   * Fetches the full details for a specific event name.
   * @param {string} eventName - The name of the event to fetch logs for.
   */
  const fetchIndividualLogs = async (eventName) => {
    try {
      const params = { event_name: eventName, ...currentSearchParams };
      individualLogs = await api.getLogs(projectId, params);

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


  // 5. Modal and Log Details Rendering

  const showModal = () => logDetailsModal.style.display = "block";
  const hideModal = () => logDetailsModal.style.display = "none";

  /**
   * Renders the detailed content for a single log inside the modal.
   * @param {number} index - The index of the log to display from the individualLogs array.
   */
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
      ...rest
    } = log;

    const primaryHTML = `
      <div class="log-details-card">
        <div class="log-details-title">${escapeHTML(event_name || "(no event name)")}</div>
        <span class="log-details-timestamp">${timestamp ? new Date(timestamp).toLocaleString() : ""}</span>
        <div class="log-details-grid">
          <div class="log-details-key">Log ID</div>
          <div class="log-details-value">${escapeHTML(id || "")}</div>
          <div class="log-details-key">Project ID</div>
          <div class="log-details-value">${escapeHTML(project_id || "")}</div>
        </div>
      </div>
    `;

    let otherFields = "";
    if (Object.keys(rest).length > 0) {
      otherFields = `
        <div class="log-details-section">
          <h4>Other Fields</h4>
          ${renderKeyValueGrid(rest)}
        </div>
      `;
    }

    logDetailsContent.innerHTML = `
      <div class="log-details-main">${primaryHTML}</div>
      <div class="log-details-section">
        <h4>Searchable Keys</h4>
        ${renderKeyValueGrid(searchable_keys)}
      </div>
      <div class="log-details-section payload-scroll-section">
        <h4>Payload</h4>
        <div class="payload-scroll">${renderKeyValueGrid(payload)}</div>
      </div>
      ${otherFields}
    `;

    updateNavButtons();
  }

  const updateNavButtons = () => {
    prevLogBtn.disabled = currentLogIndex <= 0;
    nextLogBtn.disabled = currentLogIndex >= individualLogs.length - 1;
  };

  // Helper Functions for Rendering
  const escapeHTML = (str) => {
    if (typeof str !== "string") return "";
    return str.replace(/[&<>"']/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[m]));
  };

  const renderKeyValueGrid = (obj) => {
    if (!obj || typeof obj !== "object" || Object.keys(obj).length === 0) {
      return '<div style="color:#888; font-style:italic; padding: 0.5rem;">(No data)</div>';
    }
    return (
      '<div class="log-details-grid">' +
      Object.entries(obj)
        .map(([k, v]) => `<div class="log-details-key">${escapeHTML(k)}:</div><div class="log-details-value">${escapeHTML(String(v))}</div>`)
        .join("") +
      "</div>"
    );
  };


  // 6. Event Handlers
  const handleSearch = async (e) => {
    e.preventDefault();
    const formData = new FormData(searchForm);
    const params = {
      event_name: formData.get("event_name"),
      start_time: formData.get("start_time"),
      end_time: formData.get("end_time"),
      search_keys: formData.get("search_keys"),
    };
    // Filter out empty values
    currentSearchParams = Object.fromEntries(Object.entries(params).filter(([_, v]) => v));
    await fetchAndRenderLogs(currentSearchParams);
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

  // 7. Initializer
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

  // Initial data load
  fetchAndRenderLogs({});
});