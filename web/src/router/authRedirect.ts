import type { Router } from 'vue-router'

const DEFAULT_DESTINATION = '/app'

export function safeReturnPath(router: Router, redirect: unknown): string {
    if (
        typeof redirect !== 'string' ||
        !redirect.startsWith('/') ||
        redirect.startsWith('//') ||
        redirect.includes('\\') ||
        /[\u0000-\u001f\u007f]/.test(redirect)
    ) {
        return DEFAULT_DESTINATION
    }

    try {
        const destination = router.resolve(redirect)

        // Only existing protected routes are valid login destinations.
        if (destination.matched.some((record) => record.meta.requiresAuth === true)) {
            return destination.fullPath
        }
    } catch {
        // Invalid destinations fall back to the authenticated home.
    }

    return DEFAULT_DESTINATION
}
