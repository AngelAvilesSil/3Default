import type { Router } from 'vue-router'

const DEFAULT_DESTINATION = '/'

function containsAsciiControlCharacter(value: string): boolean {
    for (let index = 0; index < value.length; index += 1) {
        const characterCode = value.charCodeAt(index)

        if (characterCode <= 0x1f || characterCode === 0x7f) {
            return true
        }
    }

    return false
}

export function safeReturnPath(router: Router, redirect: unknown): string {
    if (
        typeof redirect !== 'string' ||
        !redirect.startsWith('/') ||
        redirect.startsWith('//') ||
        redirect.includes('\\') ||
        containsAsciiControlCharacter(redirect)
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
        // Invalid destinations fall back to the public landing page.
    }

    return DEFAULT_DESTINATION
}
