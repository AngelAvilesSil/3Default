import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import App from '../App.vue'

describe('App', () => {
    it('renders the application name and router outlet', () => {
        const wrapper = mount(App, {
            global: {
                stubs: {
                    RouterView: {
                        template: '<div data-testid="router-view"></div>',
                    },
                },
            },
        })

        expect(wrapper.text()).toContain('3Default')
        expect(wrapper.find('[data-testid="router-view"]').exists()).toBe(true)
    })
})
