import { api } from './api.js';

let currentUser = null;

export async function getCurrentUser() {
    if (currentUser) return currentUser;
    try {
        currentUser = await api.getMe();
        return currentUser;
    } catch (error) {
        return null;
    }
}

export async function logout() {
    try {
        await api.logout();
    } catch (error) {
        console.error("Logout failed", error);
    } finally {
        currentUser = null;
        window.location.href = '/static/index.html';
    }
}

export function redirectToLogin() {
    window.location.href = '/static/index.html';
}

export function redirectToProjects() {
    window.location.href = '/static/projects.html';
}
