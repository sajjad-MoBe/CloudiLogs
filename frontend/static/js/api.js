const BASE_URL = '/api';

async function request(endpoint, options = {}) {
    const { body, ...customConfig } = options;
    const headers = { 'Content-Type': 'application/json' };

    const config = {
        method: body ? 'POST' : 'GET',
        ...customConfig,
        headers: {
            ...headers,
            ...customConfig.headers,
        },
    };

    if (body) {
        config.body = JSON.stringify(body);
    }

    try {
        const response = await fetch(BASE_URL + endpoint, config);
        if (!response.ok) {
            const error = await response.json();
            return Promise.reject(error);
        }
        // For 204 No Content response
        if (response.status === 204 || response.headers.get('Content-Length') === '0') {
            return null;
        }
        return response.json();
    } catch (err) {
        console.error('API call error:', err);
        return Promise.reject({ error: 'Network error or API is down.' });
    }
}

export const api = {
    login: (username, password) => request('/auth/login', { body: { username, password } }),
    logout: () => request('/auth/logout', { method: 'POST' }),
    getMe: () => request('/auth/me'),
    getProjects: () => request('/projects'),
    createProject: (projectData) => request('/projects', { body: projectData }),
    getApiKey: (projectId) => request(`/projects/${projectId}/apikey`),
};
