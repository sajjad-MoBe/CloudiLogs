import { api } from './api.js';
import { getCurrentUser, redirectToLogin } from './auth.js';

document.addEventListener('DOMContentLoaded', async () => {
    const user = await getCurrentUser();
    if (!user) {
        redirectToLogin();
        return;
    }

    const urlParams = new URLSearchParams(window.location.search);
    const projectId = urlParams.get('projectId');
    const projectName = urlParams.get('projectName');

    if (!projectId || !projectName) {
        window.location.href = '/projects.html';
        return;
    }

    // DOM Elements
    const projectNameSpan = document.getElementById('project-name');
    const searchForm = document.getElementById('search-form');
    const logsTbody = document.getElementById('logs-tbody');
    const noLogsMessage = document.getElementById('no-logs');
    const logDetailsContainer = document.getElementById('log-details-container');
    const logDetailsContent = document.getElementById('log-details-content');
    const prevLogBtn = document.getElementById('prev-log-btn');
    const nextLogBtn = document.getElementById('next-log-btn');

    // State
    let logs = [];
    let currentLogIndex = -1;
    let currentSearchParams = null;

    // Functions
    const renderLogs = () => {
        logsTbody.innerHTML = '';
        if (logs.length === 0) {
            noLogsMessage.style.display = 'block';
            logsTbody.parentElement.style.display = 'none';
        } else {
            noLogsMessage.style.display = 'none';
            logsTbody.parentElement.style.display = 'table';
            logs.forEach((log, index) => {
                const row = document.createElement('tr');
                row.innerHTML = `
                    <td>${log.event_name}</td>
                    <td>${new Date(log.timestamp).toLocaleString()}</td>
                    <td><button class="view-details-btn" data-index="${index}">View</button></td>
                `;
                logsTbody.appendChild(row);
            });
        }
    };

    const fetchAndRenderLogs = async (params) => {
        try {
            logs = await api.getLogs(projectId, params);
            renderLogs();
        } catch (error) {
            console.error('Error fetching logs:', error);
            noLogsMessage.textContent = 'Failed to load logs.';
            noLogsMessage.style.display = 'block';
        }
    };

    const showLogDetails = (index) => {
        currentLogIndex = index;
        const log = logs[index];
        logDetailsContent.textContent = JSON.stringify(log, null, 2);
        logDetailsContainer.style.display = 'block';
        updateNavButtons();
    };

    const updateNavButtons = () => {
        prevLogBtn.disabled = currentLogIndex <= 0;
        nextLogBtn.disabled = currentLogIndex >= logs.length - 1;
    };

    const handleSearch = async (e) => {
        e.preventDefault();
        const formData = new FormData(searchForm);
        const params = {
            event_name: formData.get('event_name'),
            start_time: formData.get('start_time'),
            end_time: formData.get('end_time'),
            search_keys: formData.get('search_keys'),
        };
        currentSearchParams = params;
        await fetchAndRenderLogs(params);
    };

    const handleViewDetails = (e) => {
        if (!e.target.classList.contains('view-details-btn')) return;
        const index = parseInt(e.target.dataset.index, 10);
        showLogDetails(index);
    };

    const handlePrevLog = () => {
        if (currentLogIndex > 0) {
            showLogDetails(currentLogIndex - 1);
        }
    };

    const handleNextLog = () => {
        if (currentLogIndex < logs.length - 1) {
            showLogDetails(currentLogIndex + 1);
        }
    };


    // Setup
    projectNameSpan.textContent = projectName;
    searchForm.addEventListener('submit', handleSearch);
    logsTbody.addEventListener('click', handleViewDetails);
    prevLogBtn.addEventListener('click', handlePrevLog);
    nextLogBtn.addEventListener('click', handleNextLog);

    // Initial Load
    fetchAndRenderLogs({});
});
