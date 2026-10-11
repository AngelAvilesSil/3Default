import { createRouter, createWebHistory, type Router, type RouterHistory } from 'vue-router'

import { useSessionStore } from '../stores/session'
import { safeReturnPath } from './authRedirect'

export function createAppRouter(history: RouterHistory): Router {
    const router = createRouter({
        history,
        routes: [
            {
                path: '/',
                name: 'landing',
                component: () => import('../views/LandingView.vue'),
            },
            {
                path: '/login',
                name: 'login',
                component: () => import('../views/LoginView.vue'),
            },
            {
                path: '/app',
                name: 'app-home',
                component: () => import('../views/AppHomeView.vue'),
                meta: { requiresAuth: true },
            },
            {
                path: '/session-unavailable',
                name: 'session-unavailable',
                component: () => import('../views/SessionUnavailableView.vue'),
            },
            {
                path: '/projects/:projectId/files/:projectFileId/preview',
                name: 'project-file-preview',
                component: () => import('../components/ProjectFilePreview.vue'),
                props: true,
                meta: { requiresAuth: true },
            },
        ],
    })

    router.beforeEach(async (to) => {
        // Public pages other than login never require a session check.
        if (to.name !== 'login' && to.meta.requiresAuth !== true) {
            return true
        }

        const session = useSessionStore()

        if (to.name === 'login') {
            // Detect an existing session before showing a login form.
            if (session.status === 'unknown' || session.status === 'checking') {
                try {
                    await session.restore()
                } catch {
                    // A verification failure must not prevent manual login.
                    return true
                }
            }

            if (session.status === 'authenticated') {
                return safeReturnPath(router, to.query.redirect)
            }

            return true
        }

        if (session.status !== 'authenticated' && session.status !== 'unauthenticated') {
            try {
                await session.restore()
            } catch {
                return {
                    name: 'session-unavailable',
                    query: { redirect: to.fullPath },
                }
            }
        }

        if (session.status !== 'authenticated') {
            return {
                name: 'login',
                query: { redirect: to.fullPath },
            }
        }

        return true
    })

    return router
}

const router = createAppRouter(createWebHistory(import.meta.env.BASE_URL))

export default router
