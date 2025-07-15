import { api } from './api.js';
import { getCurrentUser, redirectToProjects } from './auth.js';

document.addEventListener('DOMContentLoaded', async () => {
    // If user is already logged in, redirect to projects page
    const user = await getCurrentUser();
    if (user) {
        redirectToProjects();
        return;
    }

    const loginForm = document.getElementById('login-form');
    const errorMessage = document.getElementById('error-message');

    if (loginForm) {
        loginForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            errorMessage.textContent = '';

            const username = loginForm.username.value;
            const password = loginForm.password.value;

            try {
                await api.login(username, password);
                redirectToProjects();
            } catch (error) {
                console.error('Login error:', error);
                errorMessage.textContent = error.error || 'Login failed. Please try again.';
            }
        });
    } else {
        console.error("Login form with ID 'login-form' not found!");
    }
});
