import { describe, expect, it } from 'vitest'

import router from '../router'

describe('router', () => {
    it('defines the project file preview route as a lazy-loaded route', () => {
        const route = router
            .getRoutes()
            .find((candidate) => candidate.name === 'project-file-preview')

        expect(route).toBeDefined()
        expect(route?.path).toBe('/projects/:projectId/files/:projectFileId/preview')
        expect(route?.props.default).toBe(true)
        expect(typeof route?.components?.default).toBe('function')
    })

    it('resolves project and file IDs into the preview URL', () => {
        const resolved = router.resolve({
            name: 'project-file-preview',
            params: {
                projectId: 'project-123',
                projectFileId: 'file-456',
            },
        })

        expect(resolved.path).toBe('/projects/project-123/files/file-456/preview')
    })
})
