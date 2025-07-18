import '@testing-library/jest-dom'
import { render, screen } from '@testing-library/react'
import Home from '../../app/page'

describe('Page', () => {
    it('contains launch instruction text', async () => {
        render(await Home())

        expect(screen.getByText(/please use a launch to make this frontend functional/i)).toBeInTheDocument()
    })

    it('renders only one h1 element', async () => {
        render(await Home())

        const headings = screen.getAllByRole('heading', { level: 1 })

        expect(headings).toHaveLength(1)
    })

    it('has accessible heading structure', async () => {
        render(await Home())

        const heading = screen.getByRole('heading', { level: 1 })

        expect(heading.tagName).toBe('H1')
    })
})
