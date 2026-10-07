const GLB_MEDIA_TYPE = 'model/gltf-binary'

export type ProjectFilePreviewResult =
    | {
          status: 'available'
          data: ArrayBuffer
      }
    | {
          status: 'not-found'
      }

export class ProjectFilePreviewRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Project file preview request failed with HTTP ${status}`)

        this.name = 'ProjectFilePreviewRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class ProjectFilePreviewMediaTypeError extends Error {
    readonly mediaType: string | null

    constructor(mediaType: string | null) {
        super(`Project file preview returned an unsupported media type: ${mediaType ?? 'missing'}`)

        this.name = 'ProjectFilePreviewMediaTypeError'
        this.mediaType = mediaType
    }
}

export async function fetchProjectFilePreview(
    projectId: string,
    projectFileId: string,
    signal?: AbortSignal,
): Promise<ProjectFilePreviewResult> {
    const response = await fetch(
        `/api/projects/${encodeURIComponent(projectId)}/files/${encodeURIComponent(
            projectFileId,
        )}/preview`,
        {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: GLB_MEDIA_TYPE,
            },
            signal,
        },
    )

    if (response.status === 404) {
        return {
            status: 'not-found',
        }
    }

    if (!response.ok) {
        throw new ProjectFilePreviewRequestError(response.status, response.statusText)
    }

    const mediaType = response.headers.get('Content-Type')?.split(';', 1)[0]?.trim().toLowerCase()

    if (mediaType !== GLB_MEDIA_TYPE) {
        throw new ProjectFilePreviewMediaTypeError(mediaType ?? null)
    }

    return {
        status: 'available',
        data: await response.arrayBuffer(),
    }
}
