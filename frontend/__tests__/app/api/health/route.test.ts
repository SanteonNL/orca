/**
 * @jest-environment node
 */

import { GET } from '@/app/api/health/route'
import { NextRequest } from 'next/server'

describe('Health API Route', () => {
    it('returns status up when called', async () => {
        const request = new NextRequest('http://localhost:3000/api/health')

        const response = await GET(request)
        const data = await response.json()

        expect(data).toEqual({ status: 'up' })
    })

    it('returns 200 status code', async () => {
        const request = new NextRequest('http://localhost:3000/api/health')

        const response = await GET(request)

        expect(response.status).toBe(200)
    })

    it('returns JSON content type', async () => {
        const request = new NextRequest('http://localhost:3000/api/health')

        const response = await GET(request)

        expect(response.headers.get('content-type')).toContain('application/json')
    })

    it('handles requests with query parameters', async () => {
        const request = new NextRequest('http://localhost:3000/api/health?param=value')

        const response = await GET(request)
        const data = await response.json()

        expect(data).toEqual({ status: 'up' })
        expect(response.status).toBe(200)
    })

    it('handles requests with different HTTP headers', async () => {
        const request = new NextRequest('http://localhost:3000/api/health', {
            headers: {
                'User-Agent': 'test-agent',
                'Accept': 'application/json'
            }
        })

        const response = await GET(request)
        const data = await response.json()

        expect(data).toEqual({ status: 'up' })
    })
})