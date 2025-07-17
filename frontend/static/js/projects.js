import { api } from './api.js';
import { getCurrentUser, logout, redirectToLogin } from './auth.js';

document.addEventListener('DOMContentLoaded', async () => {
    const user = await getCurrentUser();
    if (!user) {
        redirectToLogin();
        return;
    }

    // DOM Elements
    const usernameSpan = document.getElementById('username');
    const projectsTableBody = document.querySelector('#projects-table tbody');
    const noProjectsMessage = document.getElementById('no-projects');
    const createProjectForm = document.getElementById('create-project-form');
    const createErrorMessage = document.getElementById('create-error-message');
    const logoutBtn = document.getElementById('logout-btn');

    // State
    let projects = [];

    // Functions
    const renderProjects = () => {
        projectsTableBody.innerHTML = '';
        if (projects.length === 0) {
            noProjectsMessage.style.display = 'block';
            projectsTableBody.parentElement.style.display = 'none';
        } else {
            noProjectsMessage.style.display = 'none';
            projectsTableBody.parentElement.style.display = 'table';
            projects.forEach(project => {
                const row = document.createElement('tr');
                row.innerHTML = `
                    <td><a href="/logs.html?projectId=${project.id}&projectName=${project.name}">${project.name}</a></td>
                    <td>${project.description || ''}</td>
                    <td data-project-id="${project.id}">
                        <button class="get-api-key-btn">Show API Key</button>
                    </td>
                `;
                projectsTableBody.appendChild(row);
            });
        }
    };

    const fetchAndRenderProjects = async () => {
        try {
            projects = await api.getProjects() || [];
            renderProjects();
        } catch (error) {
            console.error('Error fetching projects:', error);
            noProjectsMessage.textContent = 'Failed to load projects.';
            noProjectsMessage.style.display = 'block';
        }
    };

    const handleGetApiKey = async (e) => {
        if (!e.target.classList.contains('get-api-key-btn')) return;

        const button = e.target;
        const cell = button.parentElement;
        const projectId = cell.dataset.projectId;

        button.disabled = true;
        button.textContent = 'Loading...';

        try {
            const data = await api.getApiKey(projectId);
            cell.innerHTML = `<span class="api-key" title="Click to copy">${data.api_key}</span>`;
        } catch (error) {
            console.error('Error fetching API key:', error);
            button.textContent = 'Error';
            button.style.color = 'red';
        }
    };

    const handleCopyToClipboard = (e) => {
        if (!e.target.classList.contains('api-key')) return;

        const apiKey = e.target.textContent;
        navigator.clipboard.writeText(apiKey).then(() => {
            e.target.textContent = 'Copied!';
            setTimeout(() => {
                e.target.textContent = apiKey;
            }, 1500);
        }).catch(err => {
            console.error('Failed to copy API key:', err);
        });
    };

    const handleCreateProject = async (e) => {
        e.preventDefault();
        createErrorMessage.textContent = '';

        const formData = new FormData(createProjectForm);
        const searchableKeys = formData.get('searchable_keys').split(',').map(key => key.trim()).filter(key => key);

        const data = {
            name: formData.get('name'),
            description: formData.get('description'),
            searchable_keys: searchableKeys,
            log_ttl_seconds: parseInt(formData.get('log_ttl_seconds'), 10),
        };

        try {
            await api.createProject(data);
            createProjectForm.reset();
            await fetchAndRenderProjects();
        } catch (error) {
            console.error('Error creating project:', error);
            createErrorMessage.textContent = error.error || 'Failed to create project.';
        }
    };

    // Setup
    usernameSpan.textContent = user.username;
    logoutBtn.addEventListener('click', logout);
    createProjectForm.addEventListener('submit', handleCreateProject);
    projectsTableBody.addEventListener('click', (e) => {
        if (e.target.tagName === 'A') {
            e.preventDefault();
            window.location.href = e.target.href;
        }
        handleGetApiKey(e);
        handleCopyToClipboard(e);
    });

    // Initial Load
    fetchAndRenderProjects();
});
