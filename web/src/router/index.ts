import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: [
        {
            path: '/projects/:projectId/files/:projectFileId/preview',
            name: 'project-file-preview',
            component: () => import('../components/ProjectFilePreview.vue'),
            props: true,
        },
    ],
})

export default router
